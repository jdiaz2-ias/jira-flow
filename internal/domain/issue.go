// Package domain defines provider-independent work items and workflow contracts.
package domain

import "time"

type StatusCategory string

const (
	CategoryTodo       StatusCategory = "todo"
	CategoryInProgress StatusCategory = "in-progress"
	CategoryDone       StatusCategory = "done"
	CategoryUnknown    StatusCategory = "unknown"
)

type NamedID struct{ ID, Name string }
type User struct{ ID, DisplayName string }
type IssueRef struct{ ID, Key string }
type Status struct {
	ID, Name string
	Category StatusCategory
}

// LocalDate represents a calendar date without a UTC offset or time of day.
// Provider adapters must validate it before constructing domain values.
type LocalDate struct {
	Year  int
	Month time.Month
	Day   int
}

type Issue struct {
	Ref                             IssueRef
	Summary, ProjectID, IssueTypeID string
	Status                          Status
	Resolution                      *NamedID
	Assignee                        *User
	UpdatedAt                       time.Time
	DueDate                         *LocalDate
	URL                             string
	Priority, IssueType, Project    *NamedID
}

// Blocks contain normalized content, never terminal escapes or raw HTML.
type Block struct {
	Kind, Text, URL string
	Children        []Block
}

type IssueDetail struct {
	Issue            Issue
	Description      []Block
	Subtasks         []Issue
	SubtasksComplete bool
	Links            []IssueLink
	Comments         *CommentPage
	History          *HistoryPage
	Warnings         []string
}
type IssueLink struct {
	Type   string
	Target IssueRef
}
type DetailOptions struct {
	IncludeDescription, IncludeSubtasks, IncludeLinks   bool
	IncludeComments, IncludeHistory, All                bool
	SectionLimit, PageSize, CommentsStart, HistoryStart int
}

type SearchRequest struct {
	JQL       string
	Fields    []string
	PageSize  int
	PageToken string
}

type IssuePage struct {
	Issues        []Issue
	NextPageToken string
	Complete      bool
}

// A percentage is absent unless its denominator is known and positive.
type Progress struct {
	Status                                                      Status
	Resolution                                                  *NamedID
	SubtasksDone, SubtasksVisible                               int
	SubtasksComplete                                            bool
	SubtasksPercent                                             *float64
	TimeSpentSeconds, RemainingSeconds, OriginalEstimateSeconds *int64
	FetchedAt                                                   time.Time
	Stale                                                       bool
}

type Comment struct {
	ID        string
	Author    *User
	Body      []Block
	CreatedAt time.Time
}
type Change struct{ Field, From, To string }
type HistoryEntry struct {
	ID        string
	Author    *User
	CreatedAt time.Time
	Changes   []Change
}
type PageInfo struct {
	StartAt   int
	NextStart *int
	Total     *int
	Complete  bool
}
type CommentPage struct {
	Items []Comment
	PageInfo
}
type HistoryPage struct {
	Items []HistoryEntry
	PageInfo
}
