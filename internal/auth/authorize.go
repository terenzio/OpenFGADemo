package auth

import (
	"context"
	"fmt"
)

type Checker interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
}

func Authorize(ctx context.Context, checker Checker, relation, objectType, objectID string) error {
	userID := UserFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("no user in context")
	}

	allowed, err := checker.Check(ctx, fmt.Sprintf("user:%s", userID), relation, fmt.Sprintf("%s:%s", objectType, objectID))
	if err != nil {
		return fmt.Errorf("authorization check failed: %w", err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

var ErrForbidden = fmt.Errorf("forbidden")
