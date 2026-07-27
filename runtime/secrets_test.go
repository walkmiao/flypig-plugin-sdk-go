package runtime

import (
	"testing"
	"time"

	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
)

func TestSecretStoreRequiresDeclaredDelivery(t *testing.T) {
	_, err := NewSecretStore([]*pluginapi.SecretReference{{Name: "credential", Ref: "secret://protocol/1", Required: true}}, nil, time.Now())
	if err == nil {
		t.Fatal("required secret must be delivered")
	}
}

func TestSecretStoreCopiesAndClearsValues(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	store, err := NewSecretStore(
		[]*pluginapi.SecretReference{{Name: "credential", Ref: "secret://protocol/1", Required: true}},
		&pluginapi.SecretBundle{Values: []*pluginapi.SecretValue{{Name: "credential", Ref: "secret://protocol/1", Value: []byte("private"), ExpiresAt: &timestamp.Timestamp{Seconds: expires.Unix()}}}},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("NewSecretStore: %v", err)
	}
	leaseExpiry, ok := store.ExpiresAt("credential")
	if !ok || leaseExpiry.Unix() != expires.Unix() {
		t.Fatalf("unexpected secret expiry: %v", leaseExpiry)
	}
	value, ok := store.Bytes("credential")
	if !ok || string(value) != "private" {
		t.Fatalf("unexpected secret value: %q", value)
	}
	value[0] = 'X'
	again, _ := store.Bytes("credential")
	if string(again) != "private" {
		t.Fatal("Bytes must return a copy")
	}
	store.Close()
	if _, ok := store.Bytes("credential"); ok {
		t.Fatal("closed store must not expose values")
	}
}

func TestSecretStoreRejectsDuplicateDelivery(t *testing.T) {
	values := []*pluginapi.SecretValue{
		{Name: "credential", Ref: "secret://protocol/1", Value: []byte("first")},
		{Name: "credential", Ref: "secret://protocol/1", Value: []byte("second")},
	}
	_, err := NewSecretStore(
		[]*pluginapi.SecretReference{{Name: "credential", Ref: "secret://protocol/1", Required: true}},
		&pluginapi.SecretBundle{Values: values},
		time.Now(),
	)
	if err == nil {
		t.Fatal("duplicate secret values must be rejected")
	}
}

func TestSecretStoreRejectsExpiredDelivery(t *testing.T) {
	expires := time.Now().Add(-time.Minute)
	_, err := NewSecretStore(
		[]*pluginapi.SecretReference{{Name: "credential", Ref: "secret://protocol/1", Required: true}},
		&pluginapi.SecretBundle{Values: []*pluginapi.SecretValue{{
			Name: "credential", Ref: "secret://protocol/1", Value: []byte("private"),
			ExpiresAt: &timestamp.Timestamp{Seconds: expires.Unix()},
		}}},
		time.Now(),
	)
	if err == nil {
		t.Fatal("expired secret values must be rejected")
	}
}
