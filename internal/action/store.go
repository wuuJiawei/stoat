package action

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const maxManifestSize = 1 << 20

type Store struct{ root string }

func NewStore(root string) *Store { return &Store{root: root} }

func (s *Store) Create(operation Operation, backup []byte) (string, error) {
	if !operationIDPattern.MatchString(operation.ID) {
		return "", errors.New("invalid operation id")
	}
	directory := s.operationDir(operation.ID)
	if err := secureMkdirAll(filepath.Dir(directory)); err != nil {
		return "", err
	}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return "", fmt.Errorf("create operation directory: %w", err)
	}
	backupPath := filepath.Join(directory, "config.plist")
	if err := writeAtomic(backupPath, 0o600, func(writer io.Writer) error {
		_, err := writer.Write(backup)
		return err
	}); err != nil {
		return "", fmt.Errorf("write operation backup: %w", err)
	}
	operation.BackupPath = backupPath
	if err := s.Update(operation); err != nil {
		return "", err
	}
	return backupPath, nil
}

func (s *Store) Update(operation Operation) error {
	if !operationIDPattern.MatchString(operation.ID) {
		return errors.New("invalid operation id")
	}
	path := filepath.Join(s.operationDir(operation.ID), "manifest.json")
	return writeAtomic(path, 0o600, func(writer io.Writer) error {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(operation)
	})
}

func (s *Store) Read(operationID string) (Operation, error) {
	if !operationIDPattern.MatchString(operationID) {
		return Operation{}, errors.New("invalid operation id")
	}
	path := filepath.Join(s.operationDir(operationID), "manifest.json")
	info, err := os.Lstat(path)
	if err != nil {
		return Operation{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxManifestSize {
		return Operation{}, errors.New("refusing unsafe operation manifest")
	}
	file, err := os.Open(path)
	if err != nil {
		return Operation{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxManifestSize+1))
	decoder.DisallowUnknownFields()
	var operation Operation
	if err := decoder.Decode(&operation); err != nil {
		return Operation{}, fmt.Errorf("decode operation manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Operation{}, errors.New("operation manifest contains trailing data")
	}
	if operation.SchemaVersion != schemaVersion || operation.ID != operationID {
		return Operation{}, errors.New("operation manifest identity check failed")
	}
	return operation, nil
}

func (s *Store) List() ([]Operation, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "operations"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	operations := make([]Operation, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !operationIDPattern.MatchString(entry.Name()) {
			continue
		}
		operation, readErr := s.Read(entry.Name())
		if readErr != nil {
			return nil, readErr
		}
		operations = append(operations, operation)
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].CreatedAt.After(operations[j].CreatedAt) })
	return operations, nil
}

func (s *Store) VerifyBackup(operation Operation) error {
	cleanRoot := filepath.Clean(s.operationDir(operation.ID))
	cleanBackup := filepath.Clean(operation.BackupPath)
	if !strings.HasPrefix(cleanBackup, cleanRoot+string(os.PathSeparator)) {
		return errors.New("backup path escapes operation directory")
	}
	data, _, err := readProtectedFile(cleanBackup)
	if err != nil {
		return fmt.Errorf("read operation backup: %w", err)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != operation.Plan.ConfigSHA256 {
		return errors.New("operation backup integrity check failed")
	}
	return nil
}

func (s *Store) operationDir(operationID string) string {
	return filepath.Join(s.root, "operations", operationID)
}

func secureMkdirAll(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("state directory must be absolute")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() {
		return errors.New("state path is not a directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("state directory must be owned by the current user")
	}
	if err := os.Chmod(resolved, 0o700); err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		for current := filepath.Clean(resolved); ; current = filepath.Dir(current) {
			currentInfo, err := os.Lstat(current)
			if err != nil {
				return err
			}
			currentStat, ok := currentInfo.Sys().(*syscall.Stat_t)
			if !ok || currentStat.Uid != 0 || currentInfo.Mode().Perm()&0o022 != 0 {
				return fmt.Errorf("privileged state path contains unsafe component %s", current)
			}
			if current == string(filepath.Separator) {
				break
			}
		}
	}
	return nil
}

func writeAtomic(path string, mode os.FileMode, write func(io.Writer) error) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".stoat-tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if err := write(temporary); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}
