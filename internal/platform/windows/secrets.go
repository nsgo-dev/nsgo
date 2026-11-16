//go:build windows

package windows

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// credentials keeps an app's secrets as generic credentials of the Credential Manager, which encrypts them for the user (DPAPI): the target is "Service/Account", the user name the account and the blob the value's UTF-8 bytes, persisted as CRED_PERSIST_ENTERPRISE, roaming with the user's profile on a domain, as MyGo's App.Secrets and keytar keep them, so an app keeps reading what they wrote. Windows has no access groups or accessibility, which it ignores. Its functions may be called from any thread.
type credentials struct{}

var (
	credWriteW  = advapi32.NewProc("CredWriteW")
	credReadW   = advapi32.NewProc("CredReadW")
	credDeleteW = advapi32.NewProc("CredDeleteW")
	credFree    = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric       = 1
	credPersistEnterprise = 3
	// credMaxBlob is CRED_MAX_CREDENTIAL_BLOB_SIZE, the most bytes a credential holds.
	credMaxBlob   = 5 * 512
	errorNotFound = syscall.Errno(1168)
)

// credential is CREDENTIALW.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        [2]uint32 // FILETIME
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// credTarget is the target naming item, or why it cannot be one.
func credTarget(item driver.SecretItem) (*uint16, error) {
	return syscall.UTF16PtrFromString(item.Service + "/" + item.Account)
}

func (credentials) ReadSecret(item driver.SecretItem) (string, bool, error) {
	target, err := credTarget(item)
	if err != nil {
		return "", false, err
	}
	// The credential the system allocated, which CredFree frees.
	var c *credential
	if r, _, err := credReadW.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c))); r == 0 {
		if errors.Is(err, errorNotFound) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("credential manager: read: %w", err)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(c)))
	if c.CredentialBlobSize == 0 || c.CredentialBlob == nil {
		return "", true, nil
	}
	return string(unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize)), true, nil
}

func (credentials) WriteSecret(item driver.SecretItem, value string) error {
	if len(value) > credMaxBlob {
		return fmt.Errorf("credential manager: write: a credential holds at most %d bytes, not %d", credMaxBlob, len(value))
	}
	target, err := credTarget(item)
	if err != nil {
		return err
	}
	user, err := syscall.UTF16PtrFromString(item.Account)
	if err != nil {
		return err
	}
	blob := []byte(value)
	c := credential{Type: credTypeGeneric, TargetName: target, Persist: credPersistEnterprise, UserName: user,
		CredentialBlobSize: uint32(len(blob))}
	if len(blob) > 0 {
		c.CredentialBlob = &blob[0]
	}
	if r, _, err := credWriteW.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return fmt.Errorf("credential manager: write: %w", err)
	}
	return nil
}

func (credentials) DeleteSecret(item driver.SecretItem) error {
	target, err := credTarget(item)
	if err != nil {
		return err
	}
	if r, _, err := credDeleteW.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0); r == 0 && !errors.Is(err, errorNotFound) {
		return fmt.Errorf("credential manager: delete: %w", err)
	}
	return nil
}
