package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jeannot-muller/marstore"
	"log"
	"net/http"
	"os"
)

var c = marstore.Config{
	ClientID:         os.Getenv("Microsoft_365_APP_Client_ID"),
	ClientSecret:     os.Getenv("Microsoft_365_APP_Client_Secret"),
	TenantID:         os.Getenv("Microsoft_365_APP_Tenant_ID"),
	HostNameDev:      "localhost",
	HostNameProd:     "example.com",
	LoginPath:        "/login",                   // Url to trigger a login with M365
	LandingPath:      "/dashboard",               // Landing Page
	ErrorPath:        "/errorPage",               // Error Page (if needed / desired)
	RedirectPort:     3000,                       // Proxy Port of your go server
	RedirectPath:     "/auth/callback",           // The redirect path defined in Entra for your application
	Scope:            "openid user.read profile", // the minimum needed permissions (please maintain in Entra!)
	SessionName:      "user-session",             // the name of your session
	SessionMaxAge:    60 * 60,                    // how long your session will last
	SecurityKeyCSRF:  os.Getenv("MY_secure_string"),
	SecurityKeyStore: os.Getenv("MY_other_secure_string"),
	IsProduction:     os.Getenv("ENV") == "production",
	RedisHostName:    "127.0.0.1",
	RedisPort:        6379, // standard redis port number
	RedisPoolSize:    10,   // standard pool size, change only it you face challenges
	AllowOrigin:      []string{"*"},
	AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowHeaders:     []string{"Content-Type", "Authorization"},
}

func main() {
	marstore.InitializeStore(c) // Connect to the Redis server (configure the address and password accordingly)

	r := chi.NewRouter()
	r.Use(middleware.Logger)          // Logger needs to be the first entry to the middleware
	r.Use(middleware.Recoverer)       // CHI Recovering
	r.Use(marstore.SetupCORS(c))      // Apply CORS
	r.Use(marstore.CSRFProtection(c)) // Apply the CORS middleware

	// main route
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("Hello, World!"))
		if err != nil {
			return
		}
	})

	// login route (when typed in, you'll be forwarded to Microsoft for authentication
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		err := marstore.LoginHandler(w, r, c)
		if err != nil {
			return
		}
	})

	// Microsoft will call this path with credentials and the User struct
	r.Get("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		err := marstore.CallbackHandler(w, r, c)
		if err != nil {
			return
		}
	})

	// This is a path to trigger a logout from Microsoft, deleting as well the local cookie.
	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		err := marstore.LogoutHandler(w, r, c)
		if err != nil {
			return
		}
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(marstore.MiddlewareFactory(c))

		r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			session, err := marstore.Store.Get(r, "user-session")
			if err != nil {
				log.Printf("Error getting session in /dashboard: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Get the user info from the session
			user, ok := session.Values["user"].(marstore.Users)
			if !ok {
				log.Println("User info not found in session for /dashboard")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Use the user information here
			_, err = fmt.Fprintf(w, "Hello, %s!", user.DisplayName)
			if err != nil {
				log.Printf("Error writing response in /dashboard: %v", err)
				return
			}
			log.Printf("/dashboard accessed by user: %s", user.DisplayName)
		})

		r.Get("/other", func(w http.ResponseWriter, r *http.Request) {
			_, err := marstore.Store.Get(r, "user-session")
			if err != nil {
				log.Printf("Error getting session in /other: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			_, err = fmt.Fprintf(w, "Other page")
			if err != nil {
				log.Printf("Error writing response in /other: %v", err)
				return
			}
		})

		r.Get("/errorPage", func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte("Error!"))
			if err != nil {
				log.Printf("Error writing response in /errorPage: %v", err)
				return
			}
		})
	})

	log.Fatal(http.ListenAndServe(":3000", r))
}
