package access

import (
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

type Operator struct {
	Name string
	IPs  []string
}

type Client struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type State struct {
	Owner             *Client `json:"owner"`
	ControlAvailable bool    `json:"control_available"`
	CurrentClient    *Client `json:"current_client,omitempty"`
	LastCommandAt     string  `json:"last_command_at,omitempty"`
}

type Decision struct {
	Allowed bool
	Reason  string
	Client  *Client
	Owner   *Client
}

type Manager struct {
	logger  *slog.Logger
	timeout time.Duration

	mu              sync.Mutex
	operatorsByIP   map[string]string
	owner           *Client
	lastCommandAt   time.Time
	lastCommandMono time.Time
}

func NewManager(
	logger *slog.Logger,
	operators []Operator,
	timeout time.Duration,
) *Manager {
	m := &Manager{
		logger:        logger,
		timeout:       timeout,
		operatorsByIP: make(map[string]string),
	}

	for _, operator := range operators {
		for _, ip := range operator.IPs {
			normalized := normalizeIP(ip)
			if normalized == "" {
				continue
			}

			m.operatorsByIP[normalized] = operator.Name
		}
	}

	return m
}

func (m *Manager) CanControl(remoteAddr string) Decision {
	client := m.ResolveClient(remoteAddr)
	if client == nil {
		return Decision{
			Allowed: false,
			Reason:  "unknown_client",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.releaseExpiredLocked(time.Now())

	if m.owner == nil {
		m.owner = client
		m.lastCommandAt = time.Now()
		m.lastCommandMono = time.Now()

		m.logger.Info(
			"control owner acquired",
			"name", client.Name,
			"ip", client.IP,
		)

		return Decision{
			Allowed: true,
			Reason:  "owner_acquired",
			Client:  client,
			Owner:   cloneClient(m.owner),
		}
	}

	if sameClient(m.owner, client) {
		m.lastCommandAt = time.Now()
		m.lastCommandMono = time.Now()

		return Decision{
			Allowed: true,
			Reason:  "owner",
			Client:  client,
			Owner:   cloneClient(m.owner),
		}
	}

	return Decision{
		Allowed: false,
		Reason:  "control_busy",
		Client:  client,
		Owner:   cloneClient(m.owner),
	}
}

func (m *Manager) AllowEmergencyStop(remoteAddr string) Decision {
	client := m.ResolveClient(remoteAddr)
	if client == nil {
		return Decision{
			Allowed: false,
			Reason:  "unknown_client",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.releaseExpiredLocked(now)

	// Аварийная остановка безопасна: разрешаем любому известному пользователю.
	// Если владельца не было, этот пользователь становится владельцем.
	if m.owner == nil {
		m.owner = client
	}

	m.lastCommandAt = now
	m.lastCommandMono = now

	return Decision{
		Allowed: true,
		Reason:  "emergency_stop_allowed",
		Client:  client,
		Owner:   cloneClient(m.owner),
	}
}

func (m *Manager) ResolveClient(remoteAddr string) *Client {
	ip := ExtractIP(remoteAddr)
	if ip == "" {
		return nil
	}

	normalized := normalizeIP(ip)

	m.mu.Lock()
	defer m.mu.Unlock()

	name, ok := m.operatorsByIP[normalized]
	if !ok {
		return nil
	}

	return &Client{
		Name: name,
		IP:   normalized,
	}
}

func (m *Manager) StateFor(remoteAddr string) State {
	currentClient := m.ResolveClient(remoteAddr)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.releaseExpiredLocked(time.Now())

	state := State{
		Owner:             cloneClient(m.owner),
		ControlAvailable: m.owner == nil,
		CurrentClient:    currentClient,
	}

	if !m.lastCommandAt.IsZero() {
		state.LastCommandAt = m.lastCommandAt.Format(time.RFC3339Nano)
	}

	return state
}

func (m *Manager) ReleaseIfOwner(remoteAddr string) {
	client := m.ResolveClient(remoteAddr)
	if client == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if sameClient(m.owner, client) {
		m.logger.Info(
			"control owner released",
			"name", client.Name,
			"ip", client.IP,
		)

		m.owner = nil
		m.lastCommandAt = time.Time{}
		m.lastCommandMono = time.Time{}
	}
}

func (m *Manager) releaseExpiredLocked(now time.Time) {
	if m.owner == nil {
		return
	}

	if m.lastCommandMono.IsZero() {
		return
	}

	if now.Sub(m.lastCommandMono) <= m.timeout {
		return
	}

	m.logger.Info(
		"control owner expired",
		"name", m.owner.Name,
		"ip", m.owner.IP,
		"timeout", m.timeout.String(),
	)

	m.owner = nil
	m.lastCommandAt = time.Time{}
	m.lastCommandMono = time.Time{}
}

func ExtractIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	return remoteAddr
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return value
	}

	return ip.String()
}

func sameClient(a, b *Client) bool {
	if a == nil || b == nil {
		return false
	}

	return a.Name == b.Name && a.IP == b.IP
}

func cloneClient(client *Client) *Client {
	if client == nil {
		return nil
	}

	return &Client{
		Name: client.Name,
		IP:   client.IP,
	}
}

func (m *Manager) IsOwner(remoteAddr string) Decision {
	client := m.ResolveClient(remoteAddr)
	if client == nil {
		return Decision{
			Allowed: false,
			Reason:  "unknown_client",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.releaseExpiredLocked(time.Now())

	if sameClient(m.owner, client) {
		return Decision{
			Allowed: true,
			Reason:  "owner",
			Client:  client,
			Owner:   cloneClient(m.owner),
		}
	}

	return Decision{
		Allowed: false,
		Reason:  "not_owner",
		Client:  client,
		Owner:   cloneClient(m.owner),
	}
}