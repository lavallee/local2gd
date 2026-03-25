package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
)

// OAuth2 client credentials for an installed (desktop) application.
// These are not secret — Google's guidance for installed apps is to embed them.
// Users must create their own Google Cloud project and replace these.
var (
	ClientID     = "YOUR_CLIENT_ID.apps.googleusercontent.com"
	ClientSecret = "YOUR_CLIENT_SECRET"
)

var scopes = []string{
	drive.DriveScope,
	docs.DocumentsScope,
}

func oauthConfig(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     ClientID,
		ClientSecret: ClientSecret,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
	}
}

// Login performs the OAuth2 authorization code flow via a local browser.
// It starts a temporary HTTP server to receive the callback, opens the browser,
// and returns the resulting token.
func Login(ctx context.Context) (*oauth2.Token, error) {
	// Start listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	cfg := oauthConfig(redirectURL)

	// Generate state parameter for CSRF protection
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Channel to receive the authorization code
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch: possible CSRF attack")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("authorization denied: %s", errMsg)
			fmt.Fprintf(w, "<html><body><h1>Authorization denied</h1><p>%s</p><p>You can close this tab.</p></body></html>", errMsg)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}
		codeCh <- code
		fmt.Fprint(w, "<html><body><h1>Authorization successful!</h1><p>You can close this tab and return to the terminal.</p></body></html>")
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server error: %w", err)
		}
	}()
	defer server.Close()

	// Open browser
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	slog.Info("Opening browser for authorization...")
	fmt.Printf("If the browser doesn't open, visit this URL:\n%s\n\n", authURL)
	if err := openBrowser(authURL); err != nil {
		slog.Warn("Failed to open browser automatically", "error", err)
	}

	// Wait for callback or timeout
	select {
	case code := <-codeCh:
		token, err := cfg.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authorization timed out after 2 minutes")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TokenClient returns an HTTP client that uses the given token and auto-refreshes it.
// The onRefresh callback is called when the token is refreshed, allowing callers to persist it.
func TokenClient(ctx context.Context, token *oauth2.Token, onRefresh func(*oauth2.Token)) *http.Client {
	cfg := oauthConfig("") // redirect URL not needed for token refresh
	src := cfg.TokenSource(ctx, token)
	src = &notifyTokenSource{base: src, onRefresh: onRefresh, lastToken: token}
	return oauth2.NewClient(ctx, src)
}

// notifyTokenSource wraps a TokenSource and calls onRefresh when the token changes.
type notifyTokenSource struct {
	base      oauth2.TokenSource
	onRefresh func(*oauth2.Token)
	lastToken *oauth2.Token
}

func (n *notifyTokenSource) Token() (*oauth2.Token, error) {
	token, err := n.base.Token()
	if err != nil {
		return nil, err
	}
	if token.AccessToken != n.lastToken.AccessToken {
		n.lastToken = token
		if n.onRefresh != nil {
			n.onRefresh(token)
		}
	}
	return token, nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
