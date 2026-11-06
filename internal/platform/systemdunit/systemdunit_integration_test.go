//go:build integration

package systemdunit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
	"codeberg.org/simplicer/ataecina/internal/platform/systemdunit"
	"codeberg.org/simplicer/ataecina/internal/platform/systemdunit/systemdtest"
)

func TestStartQueuesTheUnitWithTheUserManager(t *testing.T) {
	address := dbustest.Start(t)
	manager := systemdtest.Start(t, address)
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := systemdunit.Start(context.Background(), conn, "ataecina-collect.service"); err != nil {
		t.Fatal(err)
	}
	if got := manager.Started(); !slices.Equal(got, []string{"ataecina-collect.service replace"}) {
		t.Fatalf("started = %v", got)
	}
}

func TestStartFailsWithoutAManager(t *testing.T) {
	conn, err := dbus.Connect(dbustest.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := systemdunit.Start(context.Background(), conn, "ataecina-collect.service"); err == nil {
		t.Fatal("no systemd manager on the bus must be an error")
	}
}
