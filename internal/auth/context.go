package auth

import (
	"context"

	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

// UserIDFromContext returns the authenticated user ID stored by the auth
// middleware, or false if the request is anonymous.
func UserIDFromContext(ctx context.Context) (db.UserID, bool) {
	uid, ok := ctx.Value(UserIDContextKey).(db.UserID)
	if !ok || uid == db.UserID(uuid.Nil) {
		return db.UserID(uuid.Nil), false
	}
	return uid, true
}
