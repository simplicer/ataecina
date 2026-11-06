package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	keysapp "codeberg.org/simplicer/ataecina/internal/keys/application"
	keysdomain "codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/keyfile"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/passphrase"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
	"codeberg.org/simplicer/ataecina/internal/platform/terminal"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// maxKeyBytes caps a provider key read from stdin.
const maxKeyBytes = 4096

const minimalConfig = `{
  "version": 1,
  "interval_minutes": 5,
  "subscriptions": []
}
`

// keyring is everything the keys commands need, built from the environment.
type keyring struct {
	paths     xdg.Paths
	files     keyfile.Store
	unlockers keysdomain.Unlockers
}

// openKeyring resolves the paths and the unlockers this process can use.
// passphraseFD > 0 reads passphrases from that descriptor instead of
// asking on the terminal.
func openKeyring(env environment, passphraseFD int) (keyring, error) {
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		return keyring{}, err
	}
	ask := passphrase.FromTerminal()
	if passphraseFD > 0 {
		// Read from a duplicate: the caller keeps owning its descriptor and
		// this *os.File only ever closes its own copy.
		duplicate, err := syscall.Dup(passphraseFD)
		if err != nil {
			return keyring{}, fmt.Errorf("--passphrase-fd %d: %w", passphraseFD, err)
		}
		syscall.CloseOnExec(duplicate)
		ask = passphrase.FromReader(os.NewFile(uintptr(duplicate), "passphrase"))
	}
	unlockers := keysdomain.Unlockers{keysdomain.Passphrase: passphrase.New(ask, env.passphraseIterations())}
	if env.desktopUnlockers != nil {
		for kind, unlocker := range env.desktopUnlockers(paths) {
			unlockers[kind] = unlocker
		}
	}
	return keyring{paths: paths, files: keyfile.NewStore(paths.KeysFile()), unlockers: unlockers}, nil
}

// passphraseIterations is the production PBKDF2 cost; tests may lower it.
func (env environment) passphraseIterations() int {
	if env.kdfIterations > 0 {
		return env.kdfIterations
	}
	return passphrase.Iterations
}

// passphraseFlag adds --passphrase-fd to a command that may unlock.
func passphraseFlag(flags *flag.FlagSet) *int {
	return flags.Int("passphrase-fd", 0, "read passphrases from this file descriptor instead of the terminal")
}

// initialise creates ~/.config/ataecina with safe permissions, a minimal
// config.json if there is none, and keys.json held by one unlocker.
func initialise(ctx context.Context, args []string, env environment) int {
	flags := newFlags("init", env)
	kind := flags.String("unlocker", "", "secret-service or passphrase (default: secret-service when the desktop keyring is reachable)")
	fd := passphraseFlag(flags)
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	ring, err := openKeyring(env, *fd)
	if err != nil {
		return fail(env, "init", err)
	}
	if err := privatefs.EnsureDir(ring.paths.ConfigDir()); err != nil {
		return fail(env, "init", err)
	}
	config := ring.paths.ConfigFile()
	if _, err := os.Lstat(config); errors.Is(err, os.ErrNotExist) {
		if err := privatefs.WriteFile(config, []byte(minimalConfig)); err != nil {
			return fail(env, "init", err)
		}
		fmt.Fprintf(env.stdout, "created %s\n", config)
	}
	chosen, err := chooseUnlocker(*kind, ring)
	if err != nil {
		return fail(env, "init", err)
	}
	err = keysapp.NewInitializeKeys(ring.files, aesgcm.Cipher{}, ring.unlockers).Initialize(ctx, chosen)
	if errors.Is(err, keysdomain.ErrAlreadyInitialized) {
		fmt.Fprintf(env.stdout, "%s already exists; nothing changed\n", ring.paths.KeysFile())
		return exitOK
	}
	if err != nil {
		return fail(env, "init", err)
	}
	fmt.Fprintf(env.stdout, "created %s; the master key is held by %s\n", ring.paths.KeysFile(), chosen)
	return exitOK
}

func chooseUnlocker(requested string, ring keyring) (keysdomain.UnlockerKind, error) {
	if requested != "" {
		return keysdomain.ParseUnlockerKind(requested)
	}
	if _, desktop := ring.unlockers[keysdomain.SecretService]; desktop {
		return keysdomain.SecretService, nil
	}
	return keysdomain.Passphrase, nil
}

// keysCommand manages sealed provider keys.
func keysCommand(ctx context.Context, args []string, env environment) int {
	if len(args) == 0 {
		fmt.Fprintln(env.stderr, "ataecina: keys: usage: ataecina keys set <name> | list | forget <name>")
		return exitUsage
	}
	flags := newFlags("keys "+args[0], env)
	fd := passphraseFlag(flags)
	name, rest := "", args[1:]
	if (args[0] == "set" || args[0] == "forget") && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		name, rest = rest[0], rest[1:]
	}
	if err := flags.Parse(rest); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	ring, err := openKeyring(env, *fd)
	if err != nil {
		return fail(env, "keys", err)
	}
	switch args[0] {
	case "list":
		keys, err := keysapp.NewListKeys(ring.files).List()
		if err != nil {
			return fail(env, "keys", err)
		}
		for _, key := range keys {
			fmt.Fprintf(env.stdout, "%-20s set %s\n", key.Name(), key.SetAt().Format("2006-01-02 15:04 UTC"))
		}
		return exitOK
	case "set", "forget":
		keyName, err := keysdomain.NewKeyName(name)
		if err != nil {
			fmt.Fprintf(env.stderr, "ataecina: keys: %v\n", err)
			return exitUsage
		}
		if args[0] == "forget" {
			if err := keysapp.NewForgetKey(ring.files).Forget(keyName); err != nil {
				return fail(env, "keys", err)
			}
			fmt.Fprintf(env.stdout, "forgot key %s\n", keyName)
			return exitOK
		}
		key, err := readProviderKey(env, keyName)
		if err != nil {
			return fail(env, "keys", err)
		}
		defer key.Wipe()
		if err := keysapp.NewSealKey(ring.files, aesgcm.Cipher{}, ring.unlockers, env.clock).Seal(ctx, keyName, key); err != nil {
			return fail(env, "keys", err)
		}
		fmt.Fprintf(env.stdout, "sealed key %s\n", keyName)
		return exitOK
	}
	fmt.Fprintf(env.stderr, "ataecina: keys: unknown command %q\n", args[0])
	return exitUsage
}

// readProviderKey reads a key from the terminal without echo, or from a
// pipe; never from argv or the environment.
func readProviderKey(env environment, name keysdomain.KeyName) (secret.Key, error) {
	var raw []byte
	var err error
	if env.stdinIsTerminal {
		raw, err = terminal.ReadSecret("Key for " + name.String() + " (input hidden): ")
	} else {
		raw, err = io.ReadAll(io.LimitReader(env.stdin, maxKeyBytes+1))
	}
	defer clear(raw)
	if err != nil {
		return secret.Key{}, err
	}
	trimmed := []byte(strings.TrimRight(string(raw), "\r\n"))
	defer clear(trimmed)
	if len(trimmed) == 0 || len(trimmed) > maxKeyBytes {
		return secret.Key{}, fmt.Errorf("a key is 1 to %d characters", maxKeyBytes)
	}
	for _, c := range trimmed {
		if c < 0x21 || c > 0x7e {
			return secret.Key{}, errors.New("a key is printable ASCII without spaces")
		}
	}
	return secret.New(trimmed)
}

// unlockersCommand manages how the master key is held.
func unlockersCommand(ctx context.Context, args []string, env environment) int {
	if len(args) == 0 {
		fmt.Fprintln(env.stderr, "ataecina: unlockers: usage: ataecina unlockers list | add <kind> | remove <kind>")
		return exitUsage
	}
	flags := newFlags("unlockers "+args[0], env)
	fd := passphraseFlag(flags)
	kindName, rest := "", args[1:]
	if (args[0] == "add" || args[0] == "remove") && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		kindName, rest = rest[0], rest[1:]
	}
	if err := flags.Parse(rest); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	ring, err := openKeyring(env, *fd)
	if err != nil {
		return fail(env, "unlockers", err)
	}
	if args[0] == "list" {
		kinds, err := keysapp.NewListUnlockers(ring.files).List()
		if err != nil {
			return fail(env, "unlockers", err)
		}
		for _, kind := range kinds {
			fmt.Fprintln(env.stdout, kind)
		}
		return exitOK
	}
	kind, err := keysdomain.ParseUnlockerKind(kindName)
	if err != nil || (args[0] != "add" && args[0] != "remove") {
		fmt.Fprintln(env.stderr, "ataecina: unlockers: usage: ataecina unlockers list | add <kind> | remove <kind>")
		return exitUsage
	}
	if args[0] == "add" {
		err = keysapp.NewAddUnlocker(ring.files, ring.unlockers).Add(ctx, kind)
	} else {
		err = keysapp.NewRemoveUnlocker(ring.files, ring.unlockers).Remove(ctx, kind)
	}
	if err != nil {
		return fail(env, "unlockers", err)
	}
	fmt.Fprintf(env.stdout, "%s: %s\n", args[0], kind)
	return exitOK
}

func fail(env environment, command string, err error) int {
	fmt.Fprintf(env.stderr, "ataecina: %s: %v\n", command, err)
	return exitFailure
}
