package auth

import (
	"context"
	"strconv"
	"strings"

	"quizsystem/internal/apierr"
	"quizsystem/internal/model"
	"quizsystem/internal/security"
	"quizsystem/internal/store"

	ent "quizsystem/internal/store/ent"
	entuser "quizsystem/internal/store/ent/user"

	encoreauth "encore.dev/beta/auth"
)

func init() {
	store.InitEnt(store.DB.Stdlib())
}

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (encoreauth.UID, *model.AuthData, error) {
	claims, err := security.VerifyToken(token)
	if err != nil {
		return "", nil, apierr.Unauthenticated("invalid authentication token")
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return "", nil, apierr.Unauthenticated("invalid authentication token")
	}

	u, err := store.EntClient.User.
		Query().
		Where(entuser.IDEQ(int(userID))).
		Only(ctx)
	if err != nil {
		return "", nil, apierr.Unauthenticated("invalid authentication token")
	}

	authData := model.AuthData{
		ID:    int64(u.ID),
		Email: u.Email,
		Role:  string(u.Role),
	}

	return encoreauth.UID(claims.Subject), &authData, nil
}

//encore:api public method=POST path=/auth/register
func Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	role := strings.ToLower(strings.TrimSpace(req.Role))

	if email == "" || !strings.Contains(email, "@") {
		return nil, apierr.Invalid("a valid email is required")
	}
	if len(req.Password) < 8 {
		return nil, apierr.Invalid("password must be at least 8 characters")
	}
	if role != "admin" && role != "user" {
		return nil, apierr.Invalid("role must be admin or user")
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, apierr.Internal("could not hash password", err)
	}

	u, err := store.EntClient.User.
		Create().
		SetEmail(email).
		SetPasswordHash(hash).
		SetRole(entuser.Role(role)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, apierr.AlreadyExists("email is already registered")
		}
		return nil, apierr.Internal("could not create user", err)
	}

	created := AuthUser{ID: int64(u.ID), Email: u.Email, Role: string(u.Role)}

	token, err := security.SignToken(created.ID, created.Email, created.Role)
	if err != nil {
		return nil, apierr.Internal("could not sign token", err)
	}

	return &AuthResponse{Token: token, User: created}, nil
}

//encore:api public method=POST path=/auth/login
func Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		return nil, apierr.Invalid("email and password are required")
	}

	u, err := store.EntClient.User.
		Query().
		Where(entuser.EmailEQ(email)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.Unauthenticated("invalid email or password")
		}
		return nil, apierr.Internal("could not load user", err)
	}

	if !security.VerifyPassword(req.Password, u.PasswordHash) {
		return nil, apierr.Unauthenticated("invalid email or password")
	}

	loggedIn := AuthUser{ID: int64(u.ID), Email: u.Email, Role: string(u.Role)}

	token, err := security.SignToken(loggedIn.ID, loggedIn.Email, loggedIn.Role)
	if err != nil {
		return nil, apierr.Internal("could not sign token", err)
	}

	return &AuthResponse{Token: token, User: loggedIn}, nil
}
