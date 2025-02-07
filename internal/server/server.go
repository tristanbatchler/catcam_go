package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"catcam_go/internal/db"
	"catcam_go/internal/middleware"
	"catcam_go/internal/store/users"
	"catcam_go/internal/templates"

	"github.com/a-h/templ"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const AppName = "CatCam"

type server struct {
	logger       *log.Logger
	port         int
	httpServer   *http.Server
	userStore    *users.UserStore
	sessionStore *CatCamSessionStore
}

// Creat a new server instance with the given logger and port
func NewServer(logger *log.Logger, port int, userStore *users.UserStore) (*server, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if userStore == nil {
		return nil, fmt.Errorf("userStore is required")
	}

	sessionKeyB64 := os.Getenv("SESSION_KEY")
	if sessionKeyB64 == "" {
		return nil, fmt.Errorf("SESSION_KEY is required as a base64 encoded string of 32 random bytes")
	}

	sessionKeyBytes, err := base64.StdEncoding.DecodeString(sessionKeyB64)
	if err != nil {
		return nil, fmt.Errorf("Error when decoding session key. Ensure it is a base64 encoded string of 32 random bytes: %v", err)
	}

	cookieStore := sessions.NewCookieStore(sessionKeyBytes)

	return &server{
		logger:       logger,
		port:         port,
		userStore:    userStore,
		sessionStore: NewCatCamSessionStore(cookieStore, userStore),
	}, nil
}

// Start the server
func (s *server) Start() error {
	s.logger.Printf("Starting server on port %d", s.port)
	var stopChan chan os.Signal

	// define router
	router := http.NewServeMux()

	// define middleware
	authMiddleware := middleware.Auth(s.sessionStore, s.userStore)
	loggingMiddleware := middleware.Chain(middleware.ContentType, middleware.Logging)
	authLoggingMiddleware := middleware.Chain(middleware.ContentType, middleware.Logging, authMiddleware)

	// unprotected routes:
	fileServer := http.FileServer(http.Dir("./static"))
	router.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	router.Handle("GET /favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/images/favicon/favicon.ico")
	}))

	router.Handle("GET /login", loggingMiddleware(http.HandlerFunc(s.loginFormHandler)))
	router.Handle("POST /login", loggingMiddleware(http.HandlerFunc(s.loginHandler)))

	// protected routes:
	router.Handle("GET /", authLoggingMiddleware(http.HandlerFunc(s.homeHandler)))

	router.Handle("GET /logout", authLoggingMiddleware(http.HandlerFunc(s.logoutHandler)))
	router.Handle("POST /logout", authLoggingMiddleware(http.HandlerFunc(s.logoutHandler)))

	router.Handle("POST /user", authLoggingMiddleware(http.HandlerFunc(s.addUserHandler)))
	router.Handle("GET /user/add", authLoggingMiddleware(http.HandlerFunc(s.getUserFormHandler)))
	router.Handle("DELETE /user/{id}", authLoggingMiddleware(http.HandlerFunc(s.deleteUserHandler)))
	router.Handle("GET /users", authLoggingMiddleware(http.HandlerFunc(s.listUsersHandler)))
	router.Handle("GET /user/{id}", authLoggingMiddleware(http.HandlerFunc(s.getUserHandler)))

	// define server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: router,
	}

	// create channel to listen for signals
	stopChan = make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error when running server: %s", err)
		}
	}()

	<-stopChan

	// Create a context with a timeout of 5 seconds
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Error when shutting down server: %v", err)
		return err
	}
	return nil
}

// A helper function to determine whether a request was made by HTMX, so we can use this to inform
// whether the response should be a full layout page or just the partial content
func isHtmxRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// A helper function to respond with a template, either as a full page or just the partial content
// depending on whether the request was made by HTMX and the HTML verb used (full pages only apply
// to GET requests) the AppName to the title provided. If the template fails to render, a 500 error
// is returned.
func renderTemplate(w http.ResponseWriter, r *http.Request, t templ.Component, title ...string) {
	// Return a partial response if the request was made by HTMX or if the request was not a GET request
	if isHtmxRequest(r) || r.Method != http.MethodGet {
		t.Render(r.Context(), w)
		return
	}

	// Otherwise, format the title
	if len(title) <= 0 {
		title = append(title, AppName)
	} else {
		title[0] = fmt.Sprintf("%s ~ %s", title[0], AppName)
	}

	// and render the full page
	err := templates.Layout(t, title[0]).Render(r.Context(), w)
	if err != nil {
		log.Printf("Error when rendering: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GET /
func (s *server) homeHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	renderTemplate(w, r, templates.Home(), "Home")
}

// GET /login
func (s *server) loginFormHandler(w http.ResponseWriter, r *http.Request) {
	// Pass through if already logged in
	if _, err := s.sessionStore.ValidateSession(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	renderTemplate(w, r, templates.LoginForm(nil), "Login")
}

// GET or POST /logout
func (s *server) logoutHandler(w http.ResponseWriter, r *http.Request) {

	s.sessionStore.EraseCurrent(w, r)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// POST /user
func (s *server) addUserHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Printf("Adding user")
	if err := r.ParseForm(); err != nil {
		s.logger.Printf("Error when parsing form: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	formUsername := r.FormValue("username")
	formPassword := r.FormValue("password")
	formConfirmPassword := r.FormValue("confirm-password")

	validationErrors := make(map[string]string)
	if formUsername == "" {
		validationErrors["username"] = "Username is required"
	}
	if formPassword == "" {
		validationErrors["password"] = "Password is required"
	}
	if formConfirmPassword == "" {
		validationErrors["confirm-password"] = "Confirm password is required"
	}
	if formPassword != formConfirmPassword {
		validationErrors["confirm-password"] = "Passwords do not match"
	}
	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		renderTemplate(w, r, templates.AddUserForm(db.User{Username: formUsername}, validationErrors))
		return
	}

	// Hash the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(formPassword), bcrypt.DefaultCost)
	if err != nil {
		errMsg := fmt.Sprintf("Error when hashing password: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	// Add the user to the user store
	user, err := s.userStore.AddUser(context.Background(), db.AddUserParams{
		Username:     formUsername,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		errMsg := fmt.Sprintf("Error when adding user: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	renderTemplate(w, r, templates.UserToAppend(user))
}

// GET /user/add
func (s *server) getUserFormHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, templates.AddUserForm(db.User{}, nil), "Add User")
}

// DELETE /user/{id}
func (s *server) deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Printf("Deleting user with id: %s", r.PathValue("id"))
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		errMsg := fmt.Sprintf("Error when converting id to int: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	_, err = s.userStore.DeleteUser(r.Context(), int64(id))
	if err != nil {
		errMsg := fmt.Sprintf("Error when deleting user: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	// Check if that was the last user
	numUsers, err := s.userStore.CountUsers(r.Context())
	if err != nil {
		errMsg := fmt.Sprintf("Error when counting users: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	if numUsers == 0 {
		// If we just deleted the last user, render the no users template
		renderTemplate(w, r, templates.NoUsers())
	} else {
		// Return nothing so the target of the delete request is replaced with nothing, i.e. removed
		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /users
func (s *server) listUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := s.userStore.GetUsers(r.Context())
	if err != nil {
		errMsg := fmt.Sprintf("Error when getting users: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	renderTemplate(w, r, templates.UsersList(users, s.userStore), "Users")
}

// GET /user/{id}
func (s *server) getUserHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		errMsg := fmt.Sprintf("Error when converting id to int: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	user, err := s.userStore.GetUser(r.Context(), int64(id))
	if err != nil {
		errMsg := fmt.Sprintf("Error when getting user: %v", err)
		s.logger.Print(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	renderTemplate(w, r, templates.User(user), user.Username)
}

// POST /login
func (s *server) loginHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Printf("Logging in")
	if err := r.ParseForm(); err != nil {
		s.logger.Printf("Error when parsing form: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	formUsername := r.FormValue("username")
	formPassword := r.FormValue("password")

	validationErrors := make(map[string]string)
	if formUsername == "" {
		validationErrors["username"] = "Username is required"
	}
	if formPassword == "" {
		validationErrors["password"] = "Password is required"
	}
	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		renderTemplate(w, r, templates.LoginForm(validationErrors))
		return
	}

	// Check if the user exists
	user, err := s.userStore.GetUserByUsername(r.Context(), strings.ToLower(formUsername))
	if err != nil {
		errMsg := fmt.Sprintf("Error when getting user by username: %v", err)
		switch err.(type) {
		case users.ErrUserNotFound:
			validationErrors["password"] = "Username or password is incorrect"
			w.WriteHeader(http.StatusUnauthorized)
		default:
			validationErrors["password"] = "Internal server error"
			w.WriteHeader(http.StatusInternalServerError)
		}
		s.logger.Print(errMsg)
		renderTemplate(w, r, templates.LoginForm(validationErrors))
		return
	}

	// Check if the password is correct
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(formPassword))
	if err != nil {
		validationErrors["password"] = "Username or password is incorrect"
		w.WriteHeader(http.StatusUnauthorized)
		renderTemplate(w, r, templates.LoginForm(validationErrors))
		return
	}

	// Generate a session token
	err = s.sessionStore.WriteNew(w, r, user.ID)

	if err != nil {
		errMsg := fmt.Sprintf("Error when saving session: %v", err)
		s.logger.Print(errMsg)
		w.WriteHeader(http.StatusInternalServerError)
		validationErrors["password"] = "Internal server error"
		renderTemplate(w, r, templates.LoginForm(validationErrors))
		return
	}

	s.userStore.SetUserLastLogin(r.Context(), user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
