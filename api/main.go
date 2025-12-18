package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type ContactForm struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Message        string `json:"message"`
	Honeypot       string `json:"botField"`
	TurnstileToken string `json:"turnstileToken"`
}

type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	ChallengeTs string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
}

type PostmarkEmail struct {
	From          string `json:"From"`
	To            string `json:"To"`
	Subject       string `json:"Subject"`
	TextBody      string `json:"TextBody"`
	HtmlBody      string `json:"HtmlBody"`
	MessageStream string `json:"MessageStream"`
}

type Config struct {
	PostmarkToken   string
	FromEmail       string
	ToEmail         string
	AllowedOrigin   string
	Port            string
	TurnstileSecret string
}

func loadConfig() Config {
	return Config{
		PostmarkToken:   getEnv("POSTMARK_TOKEN", ""),
		FromEmail:       getEnv("FROM_EMAIL", "noreply@traverhardwoodfloors.com"),
		ToEmail:         getEnv("TO_EMAIL", "chris@traverhardwoodfloors.com"),
		AllowedOrigin:   getEnv("ALLOWED_ORIGIN", "https://www.traverhardwoodfloors.com"),
		Port:            getEnv("PORT", "8080"),
		TurnstileSecret: getEnv("TURNSTILE_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	config := loadConfig()

	if config.PostmarkToken == "" {
		log.Fatal("POSTMARK_TOKEN environment variable is required")
	}
	if config.TurnstileSecret == "" {
		log.Fatal("TURNSTILE_SECRET environment variable is required")
	}

	http.HandleFunc("/api/contact", corsMiddleware(config.AllowedOrigin, contactHandler(config)))
	http.HandleFunc("/health", healthHandler)

	log.Printf("Server starting on port %s", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func corsMiddleware(allowedOrigin string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow the configured origin or localhost for development
		if origin == allowedOrigin || strings.HasPrefix(origin, "http://localhost") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func contactHandler(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var form ContactForm
		if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Honeypot check - if filled, silently reject (bot detected)
		if form.Honeypot != "" {
			log.Printf("Honeypot triggered, rejecting submission")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "success",
				"message": "Thank you for your message. We'll be in touch soon!",
			})
			return
		}

		// Verify Turnstile token
		if form.TurnstileToken == "" {
			http.Error(w, "Security verification required", http.StatusBadRequest)
			return
		}
		if !verifyTurnstile(config.TurnstileSecret, form.TurnstileToken, r.RemoteAddr) {
			log.Printf("Turnstile verification failed")
			http.Error(w, "Security verification failed", http.StatusForbidden)
			return
		}

		// Validate required fields
		if form.Name == "" || form.Email == "" || form.Message == "" {
			http.Error(w, "Name, email, and message are required", http.StatusBadRequest)
			return
		}

		if len(form.Name) > 50 || len(form.Email) > 50 || len(form.Phone) > 50 {
			http.Error(w, "Field length exceeded", http.StatusBadRequest)
			return
		}

		if len(form.Message) < 40 || len(form.Message) > 500 {
			http.Error(w, "Message must be between 40 and 500 characters", http.StatusBadRequest)
			return
		}

		// Send email via Postmark
		if err := sendEmail(config, form); err != nil {
			log.Printf("Error sending email: %v", err)
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Thank you for your message. We'll be in touch soon!",
		})
	}
}

func verifyTurnstile(secret, token, remoteIP string) bool {
	data := fmt.Sprintf("secret=%s&response=%s&remoteip=%s", secret, token, remoteIP)
	resp, err := http.Post(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		"application/x-www-form-urlencoded",
		strings.NewReader(data),
	)
	if err != nil {
		log.Printf("Turnstile API error: %v", err)
		return false
	}
	defer resp.Body.Close()

	var result TurnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Turnstile response parse error: %v", err)
		return false
	}

	if !result.Success {
		log.Printf("Turnstile verification failed: %v", result.ErrorCodes)
	}
	return result.Success
}

func sendEmail(config Config, form ContactForm) error {
	textBody := fmt.Sprintf(`New contact form submission from Traver Hardwood Floors website:

Name: %s
Email: %s
Phone: %s

Message:
%s
`, form.Name, form.Email, form.Phone, form.Message)

	htmlBody := fmt.Sprintf(`
<h2>New Contact Form Submission</h2>
<p><strong>Name:</strong> %s</p>
<p><strong>Email:</strong> <a href="mailto:%s">%s</a></p>
<p><strong>Phone:</strong> %s</p>
<h3>Message:</h3>
<p>%s</p>
`, form.Name, form.Email, form.Email, form.Phone, strings.ReplaceAll(form.Message, "\n", "<br>"))

	email := PostmarkEmail{
		From:          config.FromEmail,
		To:            config.ToEmail,
		Subject:       fmt.Sprintf("Contact Form: %s", form.Name),
		TextBody:      textBody,
		HtmlBody:      htmlBody,
		MessageStream: "outbound",
	}

	jsonData, err := json.Marshal(email)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.postmarkapp.com/email", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Server-Token", config.PostmarkToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("postmark returned status %d", resp.StatusCode)
	}

	return nil
}
