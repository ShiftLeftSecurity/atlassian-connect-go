package apicommunication

//    Copyright 2020 ShiftLeft Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	gojira "github.com/andygrunwald/go-jira"
	"github.com/golang-jwt/jwt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jira"

	"github.com/ShiftLeftSecurity/atlassian-connect-go/storage"
	"github.com/pkg/errors"
)

// ScopesFromStrings returns a string representing scopes in a way JIRA likes
func ScopesFromStrings(scopes []string) string {
	return strings.Join(scopes, scopeSeparator)
}

// HostClient takes it's name from the atlassian connect express code base
// where it was stolen because naming things is hard
type HostClient struct {
	ctx           context.Context
	scopes        []string
	Config        *storage.JiraInstallInformation
	UserAccountID string
	baseURL       string
	client        *http.Client
	localCache    map[string]*HostClient // more than enough for 60 sec tokens
}

// teoretically this combines DialContext and TLSHandshakeTimeout for TLS conns, we can look
// a bit more into it and define a DialTLS if necessary.
var defaultJiraTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	TLSClientConfig:       &tls.Config{},
	ExpectContinueTimeout: 1 * time.Second,
}

// NewHostClient returns a new host client for JIRA interaction based on the passed config and user account ID
func NewHostClient(ctx context.Context, config *storage.JiraInstallInformation, userAccountID string, scopes []string) (*HostClient, error) {
	return NewHostClientWithRoundtripper(ctx, config, userAccountID, scopes, defaultJiraTransport)
}

// NewHostClientWithRoundtripper is the same as NewHostClient but allows the caller to specify a custom transport
func NewHostClientWithRoundtripper(ctx context.Context, config *storage.JiraInstallInformation,
	userAccountID string, scopes []string, roundtripper http.RoundTripper) (*HostClient, error) {
	hostClient := &HostClient{
		ctx:           ctx,
		scopes:        scopes,
		Config:        config,
		UserAccountID: userAccountID,
		baseURL:       config.BaseURL,
	}
	if userAccountID != "" {
		cfg, err := getOauth2Config(ctx,
			config.BaseURL, config.OauthClientID, config.SharedSecret, userAccountID, "", scopes, "", "")
		if err != nil {
			return nil, fmt.Errorf("creating jwt config: %w", err)
		}
		hostClient.client = cfg.Client(ctx)
		return hostClient, nil
	}
	transport := gojira.JWTAuthTransport{
		Secret:    []byte(config.SharedSecret),
		Issuer:    config.Key,
		Transport: roundtripper,
	}
	hostClient.client = transport.Client()

	if config.BaseURL == "" {
		return nil, fmt.Errorf("jira install information is incomplete, base URL is empty")
	}
	hostClient.localCache = map[string]*HostClient{}
	return hostClient, nil
}

// Do performs an http action in JIRA using this client's configuration and the passed info.
func (h *HostClient) Do(method, path string, queryArgs map[string]string, body io.Reader) (*http.Response, error) {
	if h.client == nil {
		return nil, errors.Errorf("we are missing an http client")
	}

	u, err := url.Parse(h.baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "parsing jira information base URL")
	}

	u.Path = path
	q := u.Query()
	for k, v := range queryArgs {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()
	r, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "building request to JIRA")
	}
	r.Header.Add("Accept", "application/json")
	r.Header.Add("Content-Type", "application/json")
	response, err := h.client.Do(r)
	if err != nil {
		return nil, errors.Wrapf(err, "querying for %s", u.String())
	}
	return response, nil
}

// TypeFromResponse deserializes an http.Response body into an arbitrary type
// betware, this will accept anything but fail if it's not a pointer to.
func TypeFromResponse(r *http.Response, target interface{}) error {
	err := json.NewDecoder(r.Body).Decode(target)
	if err != nil {
		return errors.Wrap(err, "unmarshaling body into type")
	}
	return nil
}

// UnexpectedResponse should be returned when DoWithTarget encounters an HTTP status code that was
// not expected on a response from JIRA.
type UnexpectedResponse struct {
	obtained int
	expected []int
}

func (err *UnexpectedResponse) Error() string {
	e := make([]string, len(err.expected), len(err.expected))
	for i, ex := range err.expected {
		e[i] = strconv.Itoa(ex)
	}
	return fmt.Sprintf("obtained code %d expected one of: [%s]", err.obtained, strings.Join(e, ", "))
}

// IsUnexpectedResponse returns true if the passed error is of type UnexpectedResponse
func IsUnexpectedResponse(err error) bool {
	_, ok := err.(*UnexpectedResponse)
	return ok
}

// DoWithTarget performs a request much like do but can check for expected response codes and deserialize
// the response body into a passed target.
func (h *HostClient) DoWithTarget(method, path string, queryArgs map[string]string,
	body io.Reader, target interface{}, expectedCodes []int) (int, error) {
	resp, err := h.Do(method, path, queryArgs, body)
	if err != nil {
		return -1, fmt.Errorf("performing HTTP request: %w", err)
	}

	if len(expectedCodes) > 0 {
		for _, c := range expectedCodes {
			if resp.StatusCode == c {
				if err := TypeFromResponse(resp, target); err != nil {
					return resp.StatusCode, fmt.Errorf("deserializing result: %w", err)
				}
			}
		}
		return resp.StatusCode, &UnexpectedResponse{
			obtained: resp.StatusCode,
			expected: expectedCodes,
		}
	}
	if err := TypeFromResponse(resp, target); err != nil {
		return resp.StatusCode, fmt.Errorf("deserializing result: %w", err)
	}
	return resp.StatusCode, nil
}

const (
	// ProductTypeJira represents a jira server
	ProductTypeJira = "jira"
	// ProductTypeConfluence represents a confluence server
	ProductTypeConfluence = "confluence"
)

// AsUserByAccountID returns a HostClient whose calls impersoante another user, who is
// defined by the passed account ID
func (h *HostClient) AsUserByAccountID(userAccountID string) (*HostClient, error) {
	if userAccountID == "" {
		return nil, fmt.Errorf("user account ID must not be blank")
	}
	if chc, cached := h.localCache[userAccountID]; cached {
		// TODO: does this know how to renegotiate itself?
		return chc, nil
	}
	if strings.ToLower(h.Config.ProductType) != ProductTypeJira {
		if strings.ToLower(h.Config.ProductType) == ProductTypeConfluence {
			return nil, fmt.Errorf("the asUserByAccountID method is available for %s add-ons but this plug-in does not support it", h.Config.ProductType)
		}
		return nil, fmt.Errorf("the asUserByAccountID method is not available for %s add-ons", h.Config.ProductType)
	}
	hc, err := NewHostClient(h.ctx, h.Config, userAccountID, h.scopes)
	if err != nil {
		return nil, fmt.Errorf("creating impersonating host client: %w", err)
	}
	h.localCache[userAccountID] = hc
	return hc, nil
}

// HostClientClaims hold the necessary claims for a JIRA token
type HostClientClaims struct {
	Issuer          string `json:"iss,omitempty"`
	Audience        string `json:"sub,omitempty"`
	ExpiresIn       int64  `json:"exp,omitempty"`
	IssuedAt        int64  `json:"iat,omitempty"`
	QueryStringHash string `json:"qsh,omitempty"` // https://developer.atlassian.com/cloud/bitbucket/query-string-hash/
}

// Valid implements jwt.Claims
func (h *HostClientClaims) Valid() error {
	return nil
}

const (
	authorizationServerURL      = "https://oauth-2-authorization-server.services.atlassian.com"
	jwtClaimPrefix              = "urn:atlassian:connect"
	grantType                   = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	scopeSeparator              = " "
	defaultAuthorizationPath    = "/oauth2/token"
	defaultJWTValidityInMinutes = 3
)

func getOauth2Config(ctx context.Context,
	hostBaseURL, oauthClientID, sharedSecret, userAccountID, userKey string,
	scopes []string,
	authorizationServerBaseURL, authorizationPath string) (*jira.Config, error) {
	var userIdentifier string
	if userAccountID != "" {
		userIdentifier = userAccountID
	} else {
		userIdentifier = userKey
	}
	if authorizationServerBaseURL == "" {
		authorizationServerBaseURL = authorizationServerURL
	}
	if authorizationPath == "" {
		authorizationPath = defaultAuthorizationPath
	}

	au, err := url.Parse(authorizationServerBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing authorization server base url: %w", err)
	}
	au.Path = path.Join(au.Path, authorizationPath)
	tokenURL := au.String()

	cfg := jira.Config{
		BaseURL: hostBaseURL,
		Subject: userIdentifier,
		Config: oauth2.Config{
			ClientID:     oauthClientID,
			ClientSecret: sharedSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authorizationServerURL,
				TokenURL: tokenURL,
			},
			// Scopes are joined as a string because this is how jira acepts them
			// passing them as a list of scopes causes them to be concatenated with + symbols
			// and jira rejects the claim due to invalid scopes.
			Scopes: []string{ScopesFromStrings(scopes)},
		},
	}

	return &cfg, nil

}

// GetAccessToken performs the oauth negotiation and returns the token.
func GetAccessToken(ctx context.Context,
	hostBaseURL, oauthClientID, sharedSecret, userAccountID, userKey string,
	scopes []string,
	authorizationServerBaseURL, authorizationPath string) (*oauth2.Token, error) {

	cfg, err := getOauth2Config(ctx,
		hostBaseURL, oauthClientID, sharedSecret, userAccountID, userKey,
		scopes,
		authorizationServerBaseURL, authorizationPath)

	if err != nil {
		return nil, fmt.Errorf("getting oauth2 config: %w", err)
	}

	tokenSource := cfg.TokenSource(ctx)
	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("fetching token from atlassian: %w", err)
	}
	return token, nil
}

type jwtClaims jira.ClaimSet

func (j *jwtClaims) Valid() error {
	if j.ExpiresIn == 0 {
		return nil
	}
	t := time.Unix(j.ExpiresIn, 0)
	if time.Now().UTC().After(t) {
		return jwt.NewValidationError(fmt.Sprintf("expired in %d", j.ExpiresIn), jwt.ValidationErrorExpired)
	}
	return nil
}

// toClaims wraps jira claim set in a valid claims... why on earth would the ClaimSet not be
// compatible with jwt.Claims??
func toClaims(jcs *jira.ClaimSet) jwt.Claims {
	var validClaims *jwtClaims
	validClaims = (*jwtClaims)(jcs)
	return validClaims
}

// ValidateRequest returns jira install information for the request author if valid or error if not.
// This validation willnot work in lifecycle installed event
func ValidateRequest(r *http.Request, st storage.Store) (*storage.JiraInstallInformation, error) {
	q := r.URL.Query()
	queryJWT := q.Get("jwt")
	if queryJWT == "" {
		authHeader := r.Header.Get("Authorization")
		queryJWT = strings.TrimPrefix(authHeader, "JWT ")
		if queryJWT == "" {
			return nil, fmt.Errorf("jwt was expected in the query string or header")
		}
	}

	p := &jwt.Parser{}
	// massage a bit oauth2 claimset to be jwt.Claims friendly
	jcs := &jira.ClaimSet{}
	claims := toClaims(jcs)
	// Decode jwt to obtain info from claims
	_, _, err := p.ParseUnverified(queryJWT, claims)
	if err != nil {
		return nil, fmt.Errorf("malformed token: %w", err)
	}
	jii, err := st.JiraInstallInformation(jcs.Issuer)
	if err != nil {
		return nil, fmt.Errorf("reading jira install information from storage: %w", err)
	}
	if jii == nil {
		return nil, fmt.Errorf("no jira install information for client key: %s", jcs.Issuer)
	}
	// now validate the thing
	_, err = p.ParseWithClaims(queryJWT, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jii.SharedSecret), nil
	})
	if err != nil {
		if _, ok := err.(*jwt.ValidationError); ok {
			return nil, fmt.Errorf("malformed token: %w", err)
		}
		return nil, fmt.Errorf("parsing token: %w", err)
	}
	return jii, nil
}
