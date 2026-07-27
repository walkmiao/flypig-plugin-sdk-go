package runtime

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
)

// SecretStore exposes request-delivered secret values by logical alias. It owns
// copies of secret bytes and must be closed as soon as the request has been
// applied. A protocol driver that needs reconnect credentials may copy them into
// a task-owned in-memory lease, but must honor expires_at and zero the copy on
// replacement, stop, expiry, or shutdown. Secret values must never be logged,
// persisted, returned in errors, or copied into telemetry and extensions.
type SecretStore struct {
	values    map[string][]byte
	expiresAt map[string]time.Time
}

func NewSecretStore(refs []*pluginapi.SecretReference, bundle *pluginapi.SecretBundle, now time.Time) (*SecretStore, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	allowed := make(map[string]*pluginapi.SecretReference, len(refs))
	for _, ref := range refs {
		if ref == nil || strings.TrimSpace(ref.GetName()) == "" || strings.TrimSpace(ref.GetRef()) == "" {
			return nil, fmt.Errorf("secret reference name and ref are required")
		}
		name := strings.TrimSpace(ref.GetName())
		if _, exists := allowed[name]; exists {
			return nil, fmt.Errorf("duplicate secret reference alias %q", name)
		}
		allowed[name] = ref
	}
	store := &SecretStore{
		values:    make(map[string][]byte, len(allowed)),
		expiresAt: make(map[string]time.Time, len(allowed)),
	}
	if bundle != nil {
		seenValues := make(map[string]struct{}, len(bundle.GetValues()))
		for _, value := range bundle.GetValues() {
			if value == nil {
				continue
			}
			name := strings.TrimSpace(value.GetName())
			if _, duplicate := seenValues[name]; duplicate {
				store.Close()
				return nil, fmt.Errorf("duplicate secret value alias %q", name)
			}
			seenValues[name] = struct{}{}
			ref, exists := allowed[name]
			if !exists || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(ref.GetRef())), []byte(strings.TrimSpace(value.GetRef()))) != 1 {
				store.Close()
				return nil, fmt.Errorf("secret value %q does not match a declared reference", name)
			}
			if expires := value.GetExpiresAt(); expires != nil {
				expiresAt := time.Unix(expires.GetSeconds(), int64(expires.GetNanos())).UTC()
				if !expiresAt.After(now) {
					store.Close()
					return nil, fmt.Errorf("secret value %q is expired", name)
				}
				store.expiresAt[name] = expiresAt
			}
			store.values[name] = append([]byte(nil), value.GetValue()...)
		}
	}
	for name, ref := range allowed {
		if ref.GetRequired() && len(store.values[name]) == 0 {
			store.Close()
			return nil, fmt.Errorf("required secret %q was not delivered", name)
		}
	}
	return store, nil
}

func (s *SecretStore) Bytes(name string) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	value, ok := s.values[strings.TrimSpace(name)]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), value...), true
}

func (s *SecretStore) ExpiresAt(name string) (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	value, ok := s.expiresAt[strings.TrimSpace(name)]
	return value, ok
}

func (s *SecretStore) Close() {
	if s == nil {
		return
	}
	for name, value := range s.values {
		for index := range value {
			value[index] = 0
		}
		delete(s.values, name)
		delete(s.expiresAt, name)
	}
}
