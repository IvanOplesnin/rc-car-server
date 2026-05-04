package camera

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Proxy struct {
	logger    *slog.Logger
	streamURL string
	client    *http.Client
}

func NewProxy(streamURL string, logger *slog.Logger) *Proxy {
	return &Proxy{
		logger:    logger,
		streamURL: streamURL,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Second,
				IdleConnTimeout:       30 * time.Second,
			},
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.streamURL == "" {
		http.Error(w, "camera stream url is not configured", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.streamURL, nil)
	if err != nil {
		p.logger.Error("create camera stream request", "error", err)
		http.Error(w, "create camera stream request", http.StatusInternalServerError)
		return
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("connect to camera stream", "url", p.streamURL, "error", err)
		http.Error(w, "camera stream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.logger.Warn(
			"camera stream returned non-ok status",
			"url", p.streamURL,
			"status", resp.Status,
		)

		http.Error(w, fmt.Sprintf("camera stream returned %s", resp.Status), http.StatusBadGateway)
		return
	}

	copyCameraHeaders(w, resp)

	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)

	buf := make([]byte, 32*1024)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("camera stream client disconnected")
			return

		default:
			n, readErr := resp.Body.Read(buf)

			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					p.logger.Info("write camera stream to client", "error", writeErr)
					return
				}

				if flusher != nil {
					flusher.Flush()
				}
			}

			if readErr != nil {
				if readErr != io.EOF && ctx.Err() == nil {
					p.logger.Error("read camera stream", "error", readErr)
				}

				return
			}
		}
	}
}

func copyCameraHeaders(w http.ResponseWriter, resp *http.Response) {
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "multipart/x-mixed-replace"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Connection", "close")
}

func (p *Proxy) Check(ctx context.Context) error {
	if p.streamURL == "" {
		return fmt.Errorf("camera stream url is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.streamURL, nil)
	if err != nil {
		return fmt.Errorf("create camera check request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to camera: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("camera returned status %s", resp.Status)
	}

	return nil
}