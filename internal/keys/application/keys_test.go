package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/keys/application"
	"codeberg.org/simplicer/ataecina/internal/keys/domain"
)

var ctx = context.Background()

func passphrase() domain.Unlockers {
	return domain.Unlockers{domain.Passphrase: unlocker{kind: domain.Passphrase}}
}

func TestInitializeOnceWithAnAvailableUnlocker(t *testing.T) {
	s := &shelf{}
	if err := application.NewInitializeKeys(s, cipher{}, passphrase()).Initialize(ctx, domain.Passphrase); err != nil {
		t.Fatal(err)
	}
	if kinds, _ := application.NewListUnlockers(s).List(); len(kinds) != 1 || kinds[0] != domain.Passphrase {
		t.Fatalf("unlockers = %v", kinds)
	}
	cases := map[string]struct {
		s         *shelf
		cipher    cipher
		unlockers domain.Unlockers
		kind      domain.UnlockerKind
		want      error
	}{
		"twice":       {s, cipher{}, passphrase(), domain.Passphrase, domain.ErrAlreadyInitialized},
		"unreadable":  {&shelf{loadErr: errBoom}, cipher{}, passphrase(), domain.Passphrase, errBoom},
		"unavailable": {&shelf{}, cipher{}, passphrase(), domain.SecretService, domain.ErrUnlockerUnavailable},
		"no entropy":  {&shelf{}, cipher{newErr: errBoom}, passphrase(), domain.Passphrase, errBoom},
		"wrap fails":  {&shelf{}, cipher{}, domain.Unlockers{domain.Passphrase: unlocker{wrapErr: errBoom}}, domain.Passphrase, errBoom},
	}
	for name, c := range cases {
		if err := application.NewInitializeKeys(c.s, c.cipher, c.unlockers).Initialize(ctx, c.kind); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v, want %v", name, err, c.want)
		}
	}
}

func TestSealListUnsealAndForget(t *testing.T) {
	s := initialised(domain.Passphrase)
	if err := application.NewSealKey(s, cipher{}, passphrase(), clock{}).Seal(ctx, "zai", key("sk-zai")); err != nil {
		t.Fatal(err)
	}
	keys, err := application.NewListKeys(s).List()
	if err != nil || len(keys) != 1 || keys[0].Name() != "zai" || !keys[0].SetAt().Equal(now) {
		t.Fatalf("List = %+v, %v", keys, err)
	}
	opened, err := application.NewUnsealKey(s, cipher{}, passphrase()).Unseal(ctx, "zai")
	if err != nil || string(opened.Reveal()) != "sk-zai" {
		t.Fatalf("Unseal = %v", err)
	}
	if err := application.NewForgetKey(s).Forget("zai"); err != nil {
		t.Fatal(err)
	}
	if err := application.NewForgetKey(s).Forget("zai"); !errors.Is(err, domain.ErrKeyNotFound) {
		t.Fatalf("forgotten twice = %v", err)
	}
	for name, err := range map[string]error{
		"seal, not initialised": application.NewSealKey(&shelf{}, cipher{}, passphrase(), clock{}).Seal(ctx, "zai", key("k")),
		"seal, locked":          application.NewSealKey(initialised(domain.SecretService), cipher{}, passphrase(), clock{}).Seal(ctx, "zai", key("k")),
		"seal, cipher fails":    application.NewSealKey(initialised(domain.Passphrase), cipher{sealErr: errBoom}, passphrase(), clock{}).Seal(ctx, "zai", key("k")),
		"forget, unreadable":    application.NewForgetKey(&shelf{loadErr: errBoom}).Forget("zai"),
	} {
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	if _, err := application.NewListKeys(&shelf{}).List(); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("List without keys = %v", err)
	}
	if _, err := application.NewListUnlockers(&shelf{}).List(); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("ListUnlockers without keys = %v", err)
	}
}

func TestTheMasterKeyIsOpenedOncePerProcess(t *testing.T) {
	s := initialised(domain.SecretService, domain.Passphrase)
	_ = application.NewSealKey(s, cipher{}, passphrase(), clock{}).Seal(ctx, "zai", key("a"))
	_ = application.NewSealKey(s, cipher{}, passphrase(), clock{}).Seal(ctx, "openrouter", key("b"))
	unwraps := 0
	unlockers := domain.Unlockers{
		domain.SecretService: unlocker{kind: domain.SecretService, unwrapErr: domain.ErrUnlockerUnavailable, unwraps: &unwraps},
		domain.Passphrase:    unlocker{kind: domain.Passphrase, unwraps: &unwraps},
	}
	ring := application.NewUnsealKey(s, cipher{}, unlockers)
	var wg sync.WaitGroup
	for _, name := range []domain.KeyName{"zai", "openrouter", "zai", "openrouter"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ring.Unseal(ctx, name); err != nil {
				t.Errorf("Unseal(%s) = %v", name, err)
			}
		}()
	}
	wg.Wait()
	if unwraps != 2 {
		t.Fatalf("the master key was unwrapped %d times; want one try per unlocker, once", unwraps)
	}
	if _, err := ring.Unseal(ctx, "vercel"); !errors.Is(err, domain.ErrKeyNotFound) {
		t.Fatalf("missing key = %v", err)
	}
}

func TestALockedRingRemembersWhy(t *testing.T) {
	unlockers := domain.Unlockers{domain.Passphrase: unlocker{kind: domain.Passphrase, unwrapErr: domain.ErrBrokenSeal}}
	ring := application.NewUnsealKey(initialised(domain.Passphrase), cipher{}, unlockers)
	for range 2 {
		if _, err := ring.Unseal(ctx, "zai"); !errors.Is(err, domain.ErrLocked) || !errors.Is(err, domain.ErrBrokenSeal) {
			t.Fatalf("locked = %v", err)
		}
	}
	if _, err := application.NewUnsealKey(&shelf{}, cipher{}, unlockers).Unseal(ctx, "zai"); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("not initialised = %v", err)
	}
}

func TestUnlockersCanBeAddedAndRemoved(t *testing.T) {
	forgotten := false
	unlockers := domain.Unlockers{
		domain.Passphrase:    unlocker{kind: domain.Passphrase},
		domain.SecretService: unlocker{kind: domain.SecretService, forgotten: &forgotten},
	}
	s := initialised(domain.Passphrase)
	if err := application.NewAddUnlocker(s, unlockers).Add(ctx, domain.SecretService); err != nil {
		t.Fatal(err)
	}
	if kinds, _ := application.NewListUnlockers(s).List(); len(kinds) != 2 || kinds[0] != domain.SecretService {
		t.Fatalf("unlockers = %v", kinds)
	}
	if err := application.NewRemoveUnlocker(s, unlockers).Remove(ctx, domain.SecretService); err != nil || !forgotten {
		t.Fatalf("Remove = %v, forgotten %v", err, forgotten)
	}
	if err := application.NewRemoveUnlocker(s, unlockers).Remove(ctx, domain.Passphrase); !errors.Is(err, domain.ErrLastUnlocker) {
		t.Fatalf("last unlocker = %v", err)
	}

	addCases := map[string]struct {
		s         *shelf
		unlockers domain.Unlockers
		kind      domain.UnlockerKind
		want      error
	}{
		"not initialised": {&shelf{}, unlockers, domain.SecretService, domain.ErrNotInitialized},
		"already there":   {initialised(domain.Passphrase), unlockers, domain.Passphrase, domain.ErrUnlockerExists},
		"unavailable":     {initialised(domain.Passphrase), unlockers, domain.SystemdCreds, domain.ErrUnlockerUnavailable},
		"locked":          {initialised(domain.SystemdCreds), unlockers, domain.SecretService, domain.ErrLocked},
		"wrap fails": {initialised(domain.Passphrase), domain.Unlockers{
			domain.Passphrase: unlocker{kind: domain.Passphrase}, domain.SecretService: unlocker{wrapErr: errBoom}}, domain.SecretService, errBoom},
		"wrapped twice": {initialised(domain.Passphrase), domain.Unlockers{
			domain.Passphrase: unlocker{kind: domain.Passphrase}, domain.SecretService: unlocker{kind: domain.Passphrase}}, domain.SecretService, domain.ErrUnlockerExists},
	}
	for name, c := range addCases {
		if err := application.NewAddUnlocker(c.s, c.unlockers).Add(ctx, c.kind); !errors.Is(err, c.want) {
			t.Fatalf("add %s: %v, want %v", name, err, c.want)
		}
	}

	removeCases := map[string]struct {
		s         *shelf
		unlockers domain.Unlockers
		want      error
	}{
		"not initialised": {&shelf{}, unlockers, domain.ErrNotInitialized},
		"cannot save":     {withSaveError(initialised(domain.Passphrase, domain.SecretService)), unlockers, errBoom},
		"forget fails": {initialised(domain.Passphrase, domain.SecretService), domain.Unlockers{
			domain.SecretService: unlocker{forgetErr: errBoom}}, errBoom},
	}
	for name, c := range removeCases {
		if err := application.NewRemoveUnlocker(c.s, c.unlockers).Remove(ctx, domain.SecretService); !errors.Is(err, c.want) {
			t.Fatalf("remove %s: %v, want %v", name, err, c.want)
		}
	}
	unreachable := initialised(domain.Passphrase, domain.SecretService)
	if err := application.NewRemoveUnlocker(unreachable, passphrase()).Remove(ctx, domain.SecretService); err != nil {
		t.Fatalf("an unlocker this process cannot reach is still removed from keys.json: %v", err)
	}
}

func withSaveError(s *shelf) *shelf {
	s.saveErr = errBoom
	return s
}
