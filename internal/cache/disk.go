package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"
)

const maxDiskBytes = 25 * 1024 * 1024
const maxEntryBytes = 4 * 1024 * 1024
const maxAge = 7 * 24 * time.Hour

// Disk stores only sanitized read results supplied by the application. Filenames
// are opaque hashes. All operations are confined to v1 and serialized across processes.
type Disk struct {
	Dir, Namespace string
	Now            func() time.Time
}
type diskEntry struct {
	Version              int
	Namespace, Key, Kind string
	At                   time.Time
	TTL                  time.Duration
	Data                 json.RawMessage
}
type Stats struct {
	Entries int   `json:"entries"`
	Bytes   int64 `json:"bytes"`
	Stale   int   `json:"stale_entries"`
}

func digest(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func NewDisk(dir, namespace string) *Disk {
	return &Disk{Dir: filepath.Join(dir, "v1"), Namespace: digest(namespace), Now: time.Now}
}

var diskName = regexp.MustCompile(`^[a-f0-9]{64}\.json$`)

func (d *Disk) filename(key string) string { return digest(d.Namespace+"\x00"+key) + ".json" }

// A read/status on a missing cache does not create it. Errors carry no paths or content.
func (d *Disk) locked(ctx context.Context, create bool, fn func(*os.Root) error) error {
	if create {
		if err := os.MkdirAll(d.Dir, 0700); err != nil {
			return err
		}
	}
	info, err := os.Lstat(d.Dir)
	if errors.Is(err, os.ErrNotExist) && !create {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("cache directory must be private")
	}
	root, err := os.OpenRoot(d.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	lock, err := root.OpenFile(".lock", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if errors.Is(err, os.ErrExist) {
		info, statErr := root.Lstat(".lock")
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return errors.New("invalid cache lock")
		}
		lock, err = root.OpenFile(".lock", os.O_RDWR|syscall.O_NOFOLLOW, 0)
	}
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn(root)
}
func readEntry(root *os.Root, name string) (diskEntry, int64, error) {
	var e diskEntry
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return e, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return e, 0, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxEntryBytes {
		return e, 0, errors.New("invalid cache entry")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxEntryBytes+1))
	if err != nil {
		return e, 0, err
	}
	if json.Unmarshal(b, &e) != nil || e.Version != 1 || e.At.IsZero() || e.TTL <= 0 || e.TTL > time.Minute || len(e.Data) == 0 {
		return e, 0, errors.New("invalid cache entry")
	}
	return e, info.Size(), nil
}
func (d *Disk) Get(ctx context.Context, key string, offline bool) (data []byte, at time.Time, stale bool, err error) {
	err = d.locked(ctx, false, func(root *os.Root) error {
		e, _, err := readEntry(root, d.filename(key))
		if err != nil {
			return nil
		} // A corrupt, unsafe, or absent entry is a miss.
		age := d.Now().Sub(e.At)
		if e.Namespace != d.Namespace || e.Key != digest(key) || age < 0 || age >= maxAge {
			return nil
		}
		stale = age >= e.TTL
		if stale && !offline {
			return nil
		}
		data, at = e.Data, e.At
		return nil
	})
	return
}
func (d *Disk) Put(ctx context.Context, key, kind string, data []byte, at time.Time, ttl time.Duration) error {
	e := diskEntry{1, d.Namespace, digest(key), kind, at, ttl, data}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(b) > maxEntryBytes {
		return errors.New("cache entry exceeds size limit")
	}
	return d.locked(ctx, true, func(root *os.Root) error {
		// Fixed temporary name is safe under the inter-process lock; O_EXCL and
		// O_NOFOLLOW prevent following a pre-existing link.
		if err := root.Remove(".pending"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		f, err := root.OpenFile(".pending", os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
		if err != nil {
			return err
		}
		defer root.Remove(".pending")
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		if err = root.Rename(".pending", d.filename(key)); err != nil {
			return err
		}
		return d.prune(root)
	})
}

type storedEntry struct {
	name  string
	size  int64
	entry diskEntry
}

func entries(root *os.Root) ([]storedEntry, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	out := []storedEntry{}
	for _, name := range names {
		if !diskName.MatchString(name.Name()) {
			continue
		}
		e, size, err := readEntry(root, name.Name())
		if err != nil {
			if err = root.Remove(name.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			continue
		}
		out = append(out, storedEntry{name.Name(), size, e})
	}
	return out, nil
}
func (d *Disk) prune(root *os.Root) error {
	all, err := entries(root)
	if err != nil {
		return err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].entry.At.Before(all[j].entry.At) })
	var size int64
	details := 0
	for _, e := range all {
		size += e.size
		if e.entry.Kind == "detail" {
			details++
		}
	}
	for _, e := range all {
		age := d.Now().Sub(e.entry.At)
		if age < 0 || age >= maxAge || size > maxDiskBytes || details > 1000 {
			if err := root.Remove(e.name); err != nil {
				return err
			}
			size -= e.size
			if e.entry.Kind == "detail" {
				details--
			}
		}
	}
	return nil
}
func (d *Disk) Status(ctx context.Context) (s Stats, err error) {
	err = d.locked(ctx, false, func(root *os.Root) error {
		if err := d.prune(root); err != nil {
			return err
		}
		all, err := entries(root)
		if err != nil {
			return err
		}
		for _, e := range all {
			if e.entry.Namespace == d.Namespace {
				s.Entries++
				s.Bytes += e.size
				if d.Now().Sub(e.entry.At) >= e.entry.TTL {
					s.Stale++
				}
			}
		}
		return nil
	})
	return
}
func (d *Disk) Clear(ctx context.Context) error {
	return d.locked(ctx, false, func(root *os.Root) error {
		all, err := entries(root)
		if err != nil {
			return err
		}
		for _, e := range all {
			if e.entry.Namespace == d.Namespace {
				if err := root.Remove(e.name); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
