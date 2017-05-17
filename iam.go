package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
)

// newIAM initializes a new IAM that authenticates HTTP Requests.
// The io.Reader parameter s
func newIAM(ac accessController) (*IAM, error) {
	return &IAM{ac}, nil
}

// IAM handles request authorization for Vasco.
// It relies on the presence of an "iam-sso" cookie whose value is a JWT
// containing claims that identify a user that signs in at the IAM-SSO portal.
// Certain paths can be excluded from authorization checks through initialization
// params.
type IAM struct {
	accessController
}

// requestAuthenticator provides the interface for request authenticators.
type requestAuthenticator interface {
	authenticateRequest(req *http.Request) (*user, error)
}

var _ requestAuthenticator = (*IAM)(nil)

// authenticateRequest skips or authenticates a request.
// Skip when the request path matches a specified ACL path regex (see acl.go).
// Authenticates by checking the request header for a valid cookie value or bearer token.
// The JWT (cookie's value or auth header's bearer token) signature is checked for tampering.
func (iam *IAM) authenticateRequest(req *http.Request) (*user, error) {
	if iam.skip(req.URL.Path) {
		return nil, nil
	}
	var (
		u        *user
		err      error
		errs     []string
		flatErrs string
	)

	u, err = iam.userFromCookie(req)
	if err == nil {
		return u, nil
	}
	errs = append(errs, err.Error())

	u, err = iam.userFromAuthHeader(req)
	if err == nil {
		return u, nil
	}
	errs = append(errs, err.Error())

	for i, err := range errs {
		flatErrs += fmt.Sprintf("err%d: %s ", i, err)
	}

	return nil, fmt.Errorf("failed to authenticate: %s", flatErrs)
}

// userFromCookie returns a User if we can find the relevant cookie in the given request.
func (iam *IAM) userFromCookie(req *http.Request) (*user, error) {
	token, err := iam.extractJWTFromCookie(req)
	if err != nil {
		return nil, err
	}

	u := user{}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		u.familyName = claims["family_name"].(string)
		u.givenName = claims["given_name"].(string)
		u.email = claims["email"].(string)
		u.arn = claims["arn"].(string)
	}

	return &u, nil
}

// extractJWTFromCookie extracts the JWT from the HTTP request.
// For this to work, we rely on the presence of a cookie in the request header
// named iam-sso-* (e.g. iam-sso-dev, iam-sso-staging, iam-sso-prod) which
// indicates that the client has been to the IAM-SSO portal and that the user
// has authenticated themselves. The absence of such a cookie means they've not
// yet logged in or that their token/cookie has expired since their last login.
// Note that this is the SSO-level cookie, not an app-specific SSO cookie.
func (iam *IAM) extractJWTFromCookie(req *http.Request) (*jwt.Token, error) {
	cn, err := iam.lookupSSOCookieName(req)
	if err != nil {
		log.Printf("cannot lookup IAM SSO cookie name, falling back to 'test' because: %s", err)
		cn = "iam-sso-test"
	}

	c, err := req.Cookie(cn)
	if err != nil {
		return nil, err
	}

	return jwt.Parse(c.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return iam.lookupJWTSigningKey()
	})
}

// userFromAuthHeader returns a User if we can find the relevant authorization header in the given request.
// We look for a Bearer token who's value is the JWT.
func (iam *IAM) userFromAuthHeader(req *http.Request) (*user, error) {
	token, err := iam.extractJWTFromAuthHeader(req)
	if err != nil {
		return nil, err
	}

	u := user{}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		u.familyName = claims["family_name"].(string)
		u.givenName = claims["given_name"].(string)
		u.email = claims["email"].(string)
		u.arn = claims["arn"].(string)
	}

	return &u, nil
}

// extractJWTFromAuthHeader extracts the JWT from the HTTP request.
// For this to work, we rely on the presence of a "Bearer" Authozation header
// which indicates that the client has signed-in and obtained a token from IAM-SSO
// Note that this is the SSO-level auth header, not an app-specific one.
func (iam *IAM) extractJWTFromAuthHeader(req *http.Request) (*jwt.Token, error) {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		return nil, errors.New("expected authorization header")
	}
	if !strings.Contains(auth, "Bearer") {
		return nil, errors.New("expected Bearer")
	}
	ts := strings.SplitAfter(auth, "Bearer")[1]
	if ts == "" {
		return nil, errors.New("expected Bearer token")
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ts))
	if err != nil {
		return nil, err
	}

	return jwt.Parse(string(decoded), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return iam.lookupJWTSigningKey()
	})
}

// lookupSSOCookieName determines the kind of deploy env we're in (e.g. test, dev, staging, prod).
func (iam *IAM) lookupSSOCookieName(req *http.Request) (string, error) {
	key := "IAM_SSO_COOKIE"
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("unable to locate env var %s", key)
	}
	return val, nil
}

// lookupJWTSigning key attempts to locate the secret key
// that was used to sign our JWTs in the environment.
func (iam *IAM) lookupJWTSigningKey() ([]byte, error) {
	key := "IAM_TOKEN_SIGNING_KEY"
	val := os.Getenv(key)
	if val == "" {
		return nil, fmt.Errorf("unable to locate env var %s", key)
	}
	return []byte(val), nil
}

// user represents a user that has logged in with IAM-SSO.
type user struct {
	familyName string
	givenName  string
	email      string
	arn        string
}
