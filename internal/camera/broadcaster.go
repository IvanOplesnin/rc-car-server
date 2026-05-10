package camera

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	mjpegBoundary = "frame"
)

type Broadcaster struct {
	streamURL string
	client    *http.Client
	logger    *slog.Logger
	state     StateUpdater

	mu           sync.Mutex
	cond         *sync.Cond
	frame        []byte
	frameSeq     uint64
	connected    bool
	lastError    string
	viewersCount int
}

func NewBroadcaster(streamURL string, logger *slog.Logger, state StateUpdater,) *Broadcaster {
	b := &Broadcaster{
		streamURL: streamURL,
		state:     state,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Second,
				IdleConnTimeout:       30 * time.Second,
			},
		},
		logger: logger,
	}

	b.cond = sync.NewCond(&b.mu)

	return b
}

func (b *Broadcaster) Run(ctx context.Context) {
	b.logger.Info("camera broadcaster started", "stream_url", b.streamURL)

	defer b.logger.Info("camera broadcaster stopped")

	for {
		select {
		case <-ctx.Done():
			b.setDisconnected("context canceled")
			return
		default:
		}

		err := b.readStream(ctx)
		if err != nil {
			b.logger.Warn("camera stream read failed", "error", err)
			b.setDisconnected(err.Error())
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (b *Broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.addViewer()
	defer b.removeViewer()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+mjpegBoundary)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Connection", "close")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}

	var lastSeq uint64

	for {
		frame, seq, ok := b.waitNextFrame(r.Context(), lastSeq)
		if !ok {
			return
		}

		lastSeq = seq

		_, err := fmt.Fprintf(
			w,
			"--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n",
			mjpegBoundary,
			len(frame),
		)
		if err != nil {
			return
		}

		if _, err := w.Write(frame); err != nil {
			return
		}

		if _, err := w.Write([]byte("\r\n")); err != nil {
			return
		}

		flusher.Flush()
	}
}

func (b *Broadcaster) Status() (connected bool, lastError string, viewersCount int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.connected, b.lastError, b.viewersCount
}

func (b *Broadcaster) readStream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.streamURL, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad camera response status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("parse content type %q: %w", contentType, err)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return fmt.Errorf("missing mjpeg boundary in content type: %q", contentType)
	}

	b.setConnected()

	reader := multipart.NewReader(resp.Body, strings.TrimPrefix(boundary, "--"))

	for {
		part, err := reader.NextPart()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("camera stream closed")
			}
			return err
		}

		frame, err := io.ReadAll(part)
		_ = part.Close()

		if err != nil {
			return err
		}

		if len(frame) == 0 {
			continue
		}

		b.publishFrame(frame)
	}
}

func (b *Broadcaster) publishFrame(frame []byte) {
	copied := make([]byte, len(frame))
	copy(copied, frame)

	b.mu.Lock()
	wasConnected := b.connected
	b.frame = copied
	b.frameSeq++
	b.connected = true
	b.lastError = ""
	b.cond.Broadcast()
	b.mu.Unlock()

	if !wasConnected && b.state != nil {
		b.state.SetCameraConnected(true)
	}
}

func (b *Broadcaster) waitNextFrame(ctx context.Context, lastSeq uint64) ([]byte, uint64, bool) {
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			b.mu.Lock()
			b.cond.Broadcast()
			b.mu.Unlock()
		case <-done:
		}
	}()

	defer close(done)

	b.mu.Lock()
	defer b.mu.Unlock()

	for b.frameSeq == lastSeq {
		if ctx.Err() != nil {
			return nil, 0, false
		}

		b.cond.Wait()
	}

	frame := make([]byte, len(b.frame))
	copy(frame, b.frame)

	return frame, b.frameSeq, true
}

func (b *Broadcaster) setConnected() {
	b.mu.Lock()
	wasConnected := b.connected
	b.connected = true
	b.lastError = ""
	b.cond.Broadcast()
	b.mu.Unlock()

	if !wasConnected && b.state != nil {
		b.state.SetCameraConnected(true)
	}
}

func (b *Broadcaster) setDisconnected(reason string) {
	b.mu.Lock()
	wasConnected := b.connected
	b.connected = false
	b.lastError = reason
	b.cond.Broadcast()
	b.mu.Unlock()

	if wasConnected && b.state != nil {
		b.state.SetCameraConnected(false)
	}
}

func (b *Broadcaster) addViewer() {
	b.mu.Lock()
	b.viewersCount++
	viewersCount := b.viewersCount
	b.mu.Unlock()

	b.logger.Info("camera viewer connected", "viewers", viewersCount)
}

func (b *Broadcaster) removeViewer() {
	b.mu.Lock()
	if b.viewersCount > 0 {
		b.viewersCount--
	}
	viewersCount := b.viewersCount
	b.mu.Unlock()

	b.logger.Info("camera viewer disconnected", "viewers", viewersCount)
}
