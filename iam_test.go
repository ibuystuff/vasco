package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIAM(t *testing.T) {
	os.Setenv("IAM_TOKEN_SIGNING_KEY", "test")
	defer os.Unsetenv("IAM_TOKEN_SIGNING_KEY")

	os.Setenv("IAM_SSO_COOKIE", "iam-sso-test")
	defer os.Unsetenv("IAM_SSO_COOKIE")

	r := strings.NewReader(`[{"path": "\\/pdf\\/.*", "skip": true}]`)
	ac, err := newPathAccessController(r)
	require.NoError(t, err)

	iam, err := newIAM(ac)
	require.NoError(t, err)

	t.Run("authenticateRequestSkip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/pdf/somepath.pdf", nil)
		u, err := iam.authenticateRequest(req)
		assert.Nil(t, u)
		assert.Nil(t, err)
	})

	t.Run("authenticateRequestSuccess", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/sas/somepath", nil)
		token, signedToken := newSSOSignedTokenString(t)
		c, token := newSSOCookie(t, token, signedToken)
		req.AddCookie(c)

		u, err := iam.authenticateRequest(req)
		require.NoError(t, err)

		claims := token.Claims.(CustomClaims)
		assert.Equal(t, u.familyName, claims.FamilyName)
		assert.Equal(t, u.givenName, claims.GivenName)
		assert.Equal(t, u.email, claims.Email)
		assert.Equal(t, u.arn, claims.ARN)
	})

	t.Run("authenticateRequestForbiddenWhenNoSSOCookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/sas/somepath", nil)
		u, err := iam.authenticateRequest(req)
		assert.Nil(t, u)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "http: named cookie not present")
	})

	t.Run("authenticateRequestForbiddenWhenCompromised", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/sas/somepath", nil)
		token, signedToken := newCompromisedSSOSignedTokenString(t)
		c, _ := newSSOCookie(t, token, signedToken)
		req.AddCookie(c)

		_, err := iam.authenticateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature is invalid")
	})
}

func newSSOCookie(t *testing.T, tk *jwt.Token, ts string) (*http.Cookie, *jwt.Token) {
	dur, _ := time.ParseDuration("1m")
	cookie := new(http.Cookie)
	cookie.Name = "iam-sso-test"
	cookie.Value = ts
	cookie.Expires = time.Now().Add(dur)
	cookie.HttpOnly = true
	cookie.Path = "/"
	cookie.Domain = "127.0.0.1"
	cookie.Secure = true
	return cookie, tk
}

type CustomClaims struct {
	FamilyName string `json:"family_name"`
	GivenName  string `json:"given_name"`
	Email      string `json:"email"`
	ARN        string `json:"arn"`
	jwt.StandardClaims
}

func newSSOSignedTokenString(t *testing.T) (*jwt.Token, string) {
	claims := CustomClaims{
		"Doe",
		"John",
		"jdoe@example.com",
		"arn:iam:user:694ea0904ceaf766c6738166ed89bafb",
		jwt.StandardClaims{},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(os.Getenv("IAM_TOKEN_SIGNING_KEY")))
	if err != nil {
		t.Fatalf("can't sign token: %s", err)
	}
	return token, ss
}

func newCompromisedSSOSignedTokenString(t *testing.T) (*jwt.Token, string) {
	claims := CustomClaims{
		"Doe",
		"John",
		"jdoe@example.com",
		"arn:iam:user:694ea0904ceaf766c6738166ed89bafb",
		jwt.StandardClaims{},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte("fake signing key"))
	if err != nil {
		t.Fatalf("can't sign token: %s", err)
	}
	return token, ss
}
