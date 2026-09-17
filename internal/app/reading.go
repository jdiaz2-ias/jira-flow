package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

// ReadSession resolves credentials once; provider ports remain independent of config.
func (a Access) ReadSession(ctx context.Context, name, email string, explicit ports.Secret) (string, config.Profile, ports.Secret, error) {
	n, p, err := a.selected(name, email)
	if err != nil {
		return n, p, ports.Secret{}, err
	}
	token, _, err := a.credential(ctx, p, explicit)
	return n, p, token, err
}
func (a Access) Link(name, key string) (string, string, string, error) {
	c, err := config.Load(a.Path)
	if err != nil {
		return "", "", "", err
	}
	n, p, err := c.Select(name, a.Env)
	if err != nil {
		return "", "", "", err
	}
	if _, err = p.BaseURL(); err != nil {
		return "", "", "", err
	}
	key, err = domain.IssueKey(key)
	if err != nil {
		return "", "", "", err
	}
	link, err := domain.BrowseURL(p.SiteURL, key)
	return n, key, link, err
}

type Reader struct {
	Source                          ports.IssueReader
	Profile, Scope, ExpectedAccount string
	CursorKey                       []byte
	Cache                           *cache.Memory
	Disk                            *cache.Disk
}
type ReadMeta struct {
	Profile         string
	FetchedAt       time.Time
	Source          string
	Stale, Complete bool
	NextPageToken   string
}
type SearchOptions struct {
	Query                 domain.SearchRequest
	Limit, MaxResults     int
	All, Refresh, Offline bool
	PageToken             string
}
type SearchResult struct {
	Issues   []domain.Issue
	Meta     ReadMeta
	Warnings []string
}
type DetailResult struct {
	Detail domain.IssueDetail
	Meta   ReadMeta
}

func NewReader(source ports.IssueReader, name string, p config.Profile, token ports.Secret, memory *cache.Memory) *Reader {
	sum := sha256.Sum256([]byte(name + "\x00" + p.SiteURL + "\x00" + p.CloudID + "\x00" + p.Auth.Method + "\x00" + p.Auth.Email + "\x00" + p.AccountID + "\x00" + p.Auth.CredentialRef + "\x00" + strconv.FormatUint(p.CacheGeneration, 10) + "\x00" + token.Reveal()))
	key := sha256.Sum256([]byte("jflow-cursor-v1\x00" + token.Reveal()))
	if memory == nil {
		memory = cache.New()
	}
	return &Reader{Source: source, Profile: name, Scope: hex.EncodeToString(sum[:]), ExpectedAccount: p.AccountID, CursorKey: key[:], Cache: memory}
}
func (r *Reader) verify(ctx context.Context) error {
	user, err := r.Source.Myself(ctx)
	if err != nil {
		return err
	}
	if r.ExpectedAccount != "" && user.ID != r.ExpectedAccount {
		return problem(domain.Authentication, "Credential belongs to another identity; run auth login to update the profile.")
	}
	return nil
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type searchCursor struct {
	Version int
	Scope   string
	Token   string
	Pending []domain.Issue
	Seen    []string
	Done    bool
}

func (r *Reader) decodeCursor(raw, scope string) (searchCursor, error) {
	c := searchCursor{Version: 1, Scope: scope}
	if raw == "" {
		return c, nil
	}
	invalid := problem(domain.InvalidInput, "Invalid cursor or belongs to a different profile, identity, query, or field selection.")
	if len(raw) > 1024*1024 {
		return c, invalid
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return c, invalid
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, invalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, invalid
	}
	mac := hmac.New(sha256.New, r.CursorKey)
	mac.Write(b)
	if !hmac.Equal(sig, mac.Sum(nil)) || json.Unmarshal(b, &c) != nil || c.Version != 1 || c.Scope != scope || len(c.Seen) > 5000 || len(c.Pending) > 100 || len(c.Token) > 32768 {
		return c, invalid
	}
	return c, nil
}
func (r *Reader) encodeCursor(c searchCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", problem(domain.Internal, "Could not create the cursor.")
	}
	mac := hmac.New(sha256.New, r.CursorKey)
	mac.Write(b)
	value := base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(value) > 1024*1024 {
		return "", problem(domain.Partial, "Could not emit the cursor because it exceeds the allowed limit.")
	}
	return value, nil
}
func (r *Reader) Search(ctx context.Context, o SearchOptions) (SearchResult, error) {
	result := SearchResult{Issues: []domain.Issue{}, Meta: ReadMeta{Profile: r.Profile, Source: "network", FetchedAt: time.Now().UTC()}, Warnings: []string{}}
	if o.Limit == 0 {
		o.Limit = 50
	}
	if o.MaxResults == 0 {
		o.MaxResults = 5000
	}
	if o.Query.PageSize == 0 {
		o.Query.PageSize = 50
	}
	if o.Limit < 1 || o.Limit > 5000 || o.MaxResults < 1 || o.MaxResults > 5000 || o.Query.PageSize < 1 || o.Query.PageSize > 100 || (o.Offline && o.Refresh) {
		return result, problem(domain.InvalidInput, "Invalid limits or combination --offline --refresh not allowed.")
	}
	scope := hash([]any{r.Scope, o.Query.JQL, o.Query.Fields})
	cursor, err := r.decodeCursor(o.PageToken, scope)
	if err != nil {
		return result, err
	}
	cacheKey := r.Scope + ":search:" + hash(struct {
		Q          domain.SearchRequest
		Limit, Max int
		All        bool
		Cursor     string
	}{o.Query, o.Limit, o.MaxResults, o.All, o.PageToken})
	if !o.Refresh {
		if data, at, stale, ok := r.Cache.Get(cacheKey, o.Offline); ok {
			if json.Unmarshal(data, &result) == nil {
				result.Meta.Source = "memory"
				result.Meta.FetchedAt = at
				result.Meta.Stale = stale
				return result, nil
			}
		}
	}
	if r.Disk != nil && !o.Refresh {
		b, at, stale, e := r.Disk.Get(ctx, cacheKey, o.Offline)
		if ctx.Err() != nil {
			return result, problem(domain.Canceled, "Operation canceled.")
		}
		if e != nil {
			result.Warnings = append(result.Warnings, "Persistent cache could not be read.")
		}
		var cached SearchResult
		if len(b) > 0 && json.Unmarshal(b, &cached) == nil {
			if !o.Offline {
				if e = r.verify(ctx); e != nil {
					r.invalidate(ctx)
					return result, e
				}
			}
			cached.Meta.Source = "disk"
			cached.Meta.FetchedAt = at
			cached.Meta.Stale = stale
			return cached, nil
		}
	}
	if o.Offline {
		return result, problem(domain.NotFound, "No matching cached query. Enable cache.persist and run this exact query online first; entries expire after seven days.")
	}
	if err = r.verify(ctx); err != nil {
		r.invalidate(ctx)
		return result, err
	}
	target := o.Limit
	if o.All {
		target = o.MaxResults
	}
	seen := map[string]bool{}
	for _, id := range cursor.Seen {
		seen[id] = true
	}
	tokens := map[string]bool{}
	finish := func(cause error) (SearchResult, error) {
		result.Meta.Complete = cause == nil && cursor.Done && len(cursor.Pending) == 0
		if !result.Meta.Complete {
			var e error
			result.Meta.NextPageToken, e = r.encodeCursor(cursor)
			if e != nil && cause == nil {
				cause = e
			}
		}
		if cause == nil && o.All && !result.Meta.Complete {
			cause = problem(domain.Partial, "The guard limit was reached; the total number of results is unknown.")
		}
		if !result.Meta.Complete {
			result.Warnings = append(result.Warnings, "Incomplete result; use next_page_token with --page-token to continue.")
		}
		if cause != nil {
			r.invalidate(ctx)
			var public *domain.Error
			if errors.As(cause, &public) && public.Kind == domain.Canceled {
				return result, cause
			}
			if len(result.Issues) > 0 {
				r.invalidate(ctx)
				return result, &domain.Error{Kind: domain.Partial, Message: "Reading was incomplete: " + cause.Error(), Cause: cause}
			}
			return result, cause
		}
		if b, e := json.Marshal(result); e == nil {
			r.Cache.Put(cacheKey, b, 60*time.Second)
			if r.Disk != nil {
				if e := r.Disk.Put(ctx, cacheKey, "search", b, result.Meta.FetchedAt, time.Minute); e != nil {
					result.Warnings = append(result.Warnings, "Result could not be saved to persistent cache.")
				}
			}
		}
		return result, nil
	}
	for requests := 0; requests < 1000; {
		for len(cursor.Pending) > 0 && len(result.Issues) < target {
			if len(cursor.Seen) >= 5000 {
				return finish(problem(domain.Partial, "The limit of 5000 IDs per pagination chain was reached; start a more constrained query."))
			}
			issue := cursor.Pending[0]
			cursor.Pending = cursor.Pending[1:]
			if seen[issue.Ref.ID] {
				continue
			}
			seen[issue.Ref.ID] = true
			cursor.Seen = append(cursor.Seen, issue.Ref.ID)
			result.Issues = append(result.Issues, issue)
		}
		if (cursor.Done && len(cursor.Pending) == 0) || len(result.Issues) >= target {
			return finish(nil)
		}
		if len(cursor.Seen) >= 5000 {
			return finish(problem(domain.Partial, "The limit of 5000 IDs per pagination chain was reached; start a more constrained query."))
		}
		if tokens[cursor.Token] {
			return finish(problem(domain.Partial, "Jira repeated a pagination cursor."))
		}
		tokens[cursor.Token] = true
		q := o.Query
		q.PageSize = min(q.PageSize, target-len(result.Issues))
		q.PageToken = cursor.Token
		page, e := r.Source.Search(ctx, q)
		requests++
		if e != nil {
			return finish(e)
		}
		if len(page.Issues) > 100 {
			return finish(problem(domain.Unavailable, "Jira returned an oversized page."))
		}
		cursor.Pending = page.Issues
		cursor.Token = page.NextPageToken
		cursor.Done = page.Complete
		if !cursor.Done && cursor.Token == "" {
			return finish(problem(domain.Partial, "Cursor is missing to continue the search."))
		}
	}
	return finish(problem(domain.Partial, "The guard page limit was reached."))
}
func (r *Reader) Show(ctx context.Context, key string, o domain.DetailOptions, refresh, offline bool) (DetailResult, error) {
	result := DetailResult{Meta: ReadMeta{Profile: r.Profile, Source: "network", FetchedAt: time.Now().UTC()}}
	key, err := domain.IssueKey(key)
	if err != nil {
		return result, err
	}
	if refresh && offline {
		return result, problem(domain.InvalidInput, "--offline and --refresh are incompatible.")
	}
	cacheKey := r.Scope + ":detail:" + hash([]any{key, o})
	if !refresh {
		if b, at, stale, ok := r.Cache.Get(cacheKey, offline); ok {
			if json.Unmarshal(b, &result) == nil {
				result.Meta.Source = "memory"
				result.Meta.FetchedAt = at
				result.Meta.Stale = stale
				return result, nil
			}
		}
	}
	if r.Disk != nil && !refresh && (offline || (!o.IncludeDescription && !o.IncludeComments)) {
		b, at, stale, e := r.Disk.Get(ctx, cacheKey, offline)
		if ctx.Err() != nil {
			return result, problem(domain.Canceled, "Operation canceled.")
		}
		var cached DetailResult
		if e == nil && len(b) > 0 && json.Unmarshal(b, &cached) == nil {
			if !offline {
				if e = r.verify(ctx); e != nil {
					r.invalidate(ctx)
					return result, e
				}
			}
			cached.Meta.Source = "disk"
			cached.Meta.FetchedAt = at
			cached.Meta.Stale = stale
			if o.All && !cached.Meta.Complete {
				return cached, problem(domain.Partial, "Cached detail omits requested content.")
			}
			return cached, nil
		}
	}
	if offline {
		return result, problem(domain.NotFound, "No matching cached detail. Enable cache.persist and load this issue with the same options online first; entries expire after seven days.")
	}
	if err = r.verify(ctx); err != nil {
		r.invalidate(ctx)
		return result, err
	}
	result.Detail, err = r.Source.GetIssue(ctx, domain.IssueRef{Key: key}, o)
	result.Meta.Complete = err == nil
	if o.IncludeSubtasks && !result.Detail.SubtasksComplete {
		result.Meta.Complete = false
	}
	if c := result.Detail.Comments; c != nil && !c.Complete {
		result.Meta.Complete = false
	}
	if h := result.Detail.History; h != nil && !h.Complete {
		result.Meta.Complete = false
	}
	if err != nil {
		r.invalidate(ctx)
		return result, err
	}
	if b, e := json.Marshal(result); e == nil {
		r.Cache.Put(cacheKey, b, 30*time.Second)
		if r.Disk != nil {
			stored := result
			stored.Detail.Description = nil
			stored.Detail.Comments = nil
			stored.Detail.Values = nil
			stored.Detail.Warnings = append([]string{}, result.Detail.Warnings...)
			if h := stored.Detail.History; h != nil {
				cleaned := *h
				cleaned.Items = append([]domain.HistoryEntry{}, h.Items...)
				for i, entry := range cleaned.Items {
					cleaned.Items[i].Changes = nil
					for _, change := range entry.Changes {
						if change.FieldID == "description" || change.FieldID == "comment" || change.Field == "description" || change.Field == "comment" {
							cleaned.Complete = false
							stored.Meta.Complete = false
							continue
						}
						cleaned.Items[i].Changes = append(cleaned.Items[i].Changes, change)
					}
				}
				if !cleaned.Complete && h.Complete {
					stored.Detail.Warnings = append(stored.Detail.Warnings, "Description/comment changes are excluded from cached history.")
				}
				stored.Detail.History = &cleaned
			}
			if o.IncludeDescription || o.IncludeComments {
				stored.Meta.Complete = false
				stored.Detail.Warnings = append(stored.Detail.Warnings, "Description and comment bodies are excluded from persistent cache; use an online query to load them.")
			}
			b, e := json.Marshal(stored)
			if e == nil {
				e = r.Disk.Put(ctx, cacheKey, "detail", b, result.Meta.FetchedAt, 30*time.Second)
			}
			if e != nil {
				result.Detail.Warnings = append(result.Detail.Warnings, "Detail could not be saved to persistent cache.")
			}
		}
	}
	return result, nil
}

func (r *Reader) invalidate(ctx context.Context) {
	r.Cache.DeletePrefix(r.Scope + ":")
	if r.Disk != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = r.Disk.Clear(cleanup)
	}
}
