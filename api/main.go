package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
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

type PostmarkAttachment struct {
	Name        string `json:"Name"`
	Content     string `json:"Content"`
	ContentType string `json:"ContentType"`
}

type PostmarkEmail struct {
	From          string              `json:"From"`
	To            string              `json:"To"`
	Subject       string              `json:"Subject"`
	TextBody      string              `json:"TextBody"`
	HtmlBody      string              `json:"HtmlBody"`
	MessageStream string              `json:"MessageStream"`
	Attachments   []PostmarkAttachment `json:"Attachments,omitempty"`
}

type Config struct {
	PostmarkToken   string
	FromEmail       string
	ToEmail         string
	AllowedOrigin   string
	Port            string
	TurnstileSecret string
	SentryDSN       string
}

func loadConfig() Config {
	return Config{
		PostmarkToken:   getEnv("POSTMARK_TOKEN", ""),
		FromEmail:       getEnv("FROM_EMAIL", "noreply@traverhardwoodfloors.com"),
		ToEmail:         getEnv("TO_EMAIL", "chris@traverhardwoodfloors.com"),
		AllowedOrigin:   getEnv("ALLOWED_ORIGIN", "https://www.traverhardwoodfloors.com"),
		Port:            getEnv("API_PORT", "8080"),
		TurnstileSecret: getEnv("TURNSTILE_SECRET", ""),
		SentryDSN:       getEnv("SENTRY_DSN", ""),
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

	// Initialize Sentry for error tracking (optional)
	if config.SentryDSN == "" {
		log.Println("SENTRY_DSN not set, error tracking disabled")
	} else {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              config.SentryDSN,
			SendDefaultPII:   true,
			TracesSampleRate: 0.0,
		}); err != nil {
			log.Printf("Sentry init failed: %v", err)
		} else {
			log.Println("Error tracking enabled")
		}
		defer sentry.Flush(2 * time.Second)
	}

	http.HandleFunc("/api/contact", recoverMiddleware(corsMiddleware(config.AllowedOrigin, contactHandler(config))))
	http.HandleFunc("/api/estimate", recoverMiddleware(corsMiddleware(config.AllowedOrigin, estimateHandler(config))))
	http.HandleFunc("/health", recoverMiddleware(healthHandler))

	log.Printf("Server starting on port %s", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				sentry.CurrentHub().Recover(err)
				sentry.Flush(2 * time.Second)
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

func corsMiddleware(allowedOrigin string, next http.HandlerFunc) http.HandlerFunc {
	// Build set of allowed origins: configured origin + www/non-www variant
	allowed := map[string]bool{allowedOrigin: true}
	if strings.Contains(allowedOrigin, "://www.") {
		allowed[strings.Replace(allowedOrigin, "://www.", "://", 1)] = true
	} else {
		allowed[strings.Replace(allowedOrigin, "://", "://www.", 1)] = true
	}

	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow the configured origin (www and non-www) or localhost for development
		if allowed[origin] || strings.HasPrefix(origin, "http://localhost") {
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
		turnstileOK, err := verifyTurnstile(config.TurnstileSecret, form.TurnstileToken, r.RemoteAddr)
		if err != nil {
			log.Printf("Turnstile infrastructure error: %v", err)
			sentry.CaptureException(err)
			http.Error(w, "Security verification unavailable", http.StatusServiceUnavailable)
			return
		}
		if !turnstileOK {
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
			sentry.CaptureException(err)
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

func verifyTurnstile(secret, token, remoteIP string) (bool, error) {
	data := fmt.Sprintf("secret=%s&response=%s&remoteip=%s", secret, token, remoteIP)
	resp, err := http.Post(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		"application/x-www-form-urlencoded",
		strings.NewReader(data),
	)
	if err != nil {
		return false, fmt.Errorf("turnstile API request failed: %w", err)
	}
	defer resp.Body.Close()

	var result TurnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("turnstile response parse error: %w", err)
	}

	if !result.Success {
		log.Printf("Turnstile verification failed: %v", result.ErrorCodes)
	}
	return result.Success, nil
}

var allowedProjectTypes = map[string]bool{
	"installation": true,
	"refinishing":  true,
	"repair":       true,
	"stairs":       true,
	"other":        true,
}

func estimateHandler(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 12<<20) // 12MB

		if err := r.ParseMultipartForm(12 << 20); err != nil {
			http.Error(w, "Request too large or invalid form data", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		email := strings.TrimSpace(r.FormValue("email"))
		phone := strings.TrimSpace(r.FormValue("phone"))
		city := strings.TrimSpace(r.FormValue("city"))
		projectType := strings.TrimSpace(r.FormValue("projectType"))
		squareFootage := strings.TrimSpace(r.FormValue("squareFootage"))
		message := strings.TrimSpace(r.FormValue("message"))
		honeypot := strings.TrimSpace(r.FormValue("botField"))
		turnstileToken := strings.TrimSpace(r.FormValue("turnstileToken"))

		// Honeypot check
		if honeypot != "" {
			log.Printf("Honeypot triggered on estimate form, rejecting")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "success",
				"message": "Thank you! We'll be in touch soon to schedule your estimate.",
			})
			return
		}

		// Turnstile verification
		if turnstileToken == "" {
			http.Error(w, "Security verification required", http.StatusBadRequest)
			return
		}
		turnstileOK, err := verifyTurnstile(config.TurnstileSecret, turnstileToken, r.RemoteAddr)
		if err != nil {
			log.Printf("Turnstile infrastructure error: %v", err)
			sentry.CaptureException(err)
			http.Error(w, "Security verification unavailable", http.StatusServiceUnavailable)
			return
		}
		if !turnstileOK {
			log.Printf("Turnstile verification failed on estimate form")
			http.Error(w, "Security verification failed", http.StatusForbidden)
			return
		}

		// Validate required fields
		if name == "" || email == "" || city == "" || message == "" {
			http.Error(w, "Name, email, city, and message are required", http.StatusBadRequest)
			return
		}
		if len(name) > 50 || len(email) > 50 || len(phone) > 50 || len(city) > 100 {
			http.Error(w, "Field length exceeded", http.StatusBadRequest)
			return
		}
		if len(message) < 40 || len(message) > 1000 {
			http.Error(w, "Message must be between 40 and 1000 characters", http.StatusBadRequest)
			return
		}
		if projectType != "" && !allowedProjectTypes[projectType] {
			http.Error(w, "Invalid project type", http.StatusBadRequest)
			return
		}
		if len(squareFootage) > 20 {
			http.Error(w, "Field length exceeded", http.StatusBadRequest)
			return
		}

		// Process photo attachments
		var attachments []PostmarkAttachment
		if r.MultipartForm != nil && r.MultipartForm.File["photos"] != nil {
			files := r.MultipartForm.File["photos"]
			if len(files) > 5 {
				http.Error(w, "Maximum 5 photos allowed", http.StatusBadRequest)
				return
			}
			for _, fh := range files {
				if fh.Size > 2<<20 { // 2MB per file
					http.Error(w, "Each photo must be under 2MB", http.StatusBadRequest)
					return
				}
				ct := fh.Header.Get("Content-Type")
				if ct != "image/jpeg" && ct != "image/png" && ct != "image/webp" {
					http.Error(w, "Photos must be JPEG, PNG, or WebP", http.StatusBadRequest)
					return
				}
				f, err := fh.Open()
				if err != nil {
					log.Printf("Error opening uploaded file: %v", err)
					http.Error(w, "Error processing upload", http.StatusInternalServerError)
					return
				}
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil {
					log.Printf("Error reading uploaded file: %v", err)
					http.Error(w, "Error processing upload", http.StatusInternalServerError)
					return
				}
				attachments = append(attachments, PostmarkAttachment{
					Name:        fh.Filename,
					Content:     base64.StdEncoding.EncodeToString(data),
					ContentType: ct,
				})
			}
		}

		if err := sendEstimateEmail(config, r, attachments); err != nil {
			log.Printf("Error sending estimate email: %v", err)
			sentry.CaptureException(err)
			http.Error(w, "Failed to send estimate request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Thank you! We'll be in touch soon to schedule your estimate.",
		})
	}
}

func sendEstimateEmail(config Config, r *http.Request, attachments []PostmarkAttachment) error {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	city := strings.TrimSpace(r.FormValue("city"))
	projectType := strings.TrimSpace(r.FormValue("projectType"))
	squareFootage := strings.TrimSpace(r.FormValue("squareFootage"))
	message := strings.TrimSpace(r.FormValue("message"))

	photoLine := ""
	if len(attachments) > 0 {
		photoLine = fmt.Sprintf("\n%d photo(s) attached", len(attachments))
	}

	textBody := fmt.Sprintf(`New estimate request from Traver Hardwood Floors website:

Name: %s
Email: %s
Phone: %s
City / Town: %s
Project Type: %s
Approx. Square Footage: %s

Message:
%s
%s`, name, email, phone, city, projectType, squareFootage, message, photoLine)

	photoHTML := ""
	if len(attachments) > 0 {
		photoHTML = fmt.Sprintf(`<p><strong>Photos:</strong> %d photo(s) attached</p>`, len(attachments))
	}

	htmlBody := fmt.Sprintf(`
<h2>New Estimate Request</h2>
<table style="border-collapse:collapse;width:100%%;max-width:600px;">
<tr><td style="padding:8px;border-bottom:1px solid #eee;font-weight:bold;">Name</td><td style="padding:8px;border-bottom:1px solid #eee;">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #eee;font-weight:bold;">Email</td><td style="padding:8px;border-bottom:1px solid #eee;"><a href="mailto:%s">%s</a></td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #eee;font-weight:bold;">Phone</td><td style="padding:8px;border-bottom:1px solid #eee;">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #eee;font-weight:bold;">City / Town</td><td style="padding:8px;border-bottom:1px solid #eee;">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #eee;font-weight:bold;">Project Type</td><td style="padding:8px;border-bottom:1px solid #eee;">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #eee;font-weight:bold;">Approx. Sq Ft</td><td style="padding:8px;border-bottom:1px solid #eee;">%s</td></tr>
</table>
<h3>Message:</h3>
<p>%s</p>
%s`,
		name, email, email, phone, city, projectType, squareFootage,
		strings.ReplaceAll(message, "\n", "<br>"), photoHTML)

	postmarkEmail := PostmarkEmail{
		From:          config.FromEmail,
		To:            config.ToEmail,
		Subject:       fmt.Sprintf("Estimate Request: %s — %s", name, city),
		TextBody:      textBody,
		HtmlBody:      htmlBody,
		MessageStream: "outbound",
		Attachments:   attachments,
	}

	jsonData, err := json.Marshal(postmarkEmail)
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
