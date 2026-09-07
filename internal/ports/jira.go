// Package ports declares application boundaries; implementations live in adapters.
package ports

import (
	"context"
	"jira-flow.local/jflow/internal/domain"
)

type IssueReader interface {
	Myself(context.Context) (domain.User, error)
	Search(context.Context, domain.SearchRequest) (domain.IssuePage, error)
	GetIssue(context.Context, domain.IssueRef, domain.DetailOptions) (domain.IssueDetail, error)
}

type TransitionGateway interface {
	ListTransitions(context.Context, domain.IssueRef) ([]domain.Transition, error)
	ApplyTransition(context.Context, domain.TransitionRequest) (domain.ApplyResult, error)
}
