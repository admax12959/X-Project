// Package apitest is the shared HTTP API test harness: a real MySQL 8 server,
// a fresh database per test, and the production startup path (migrate, bootstrap)
// behind an httptest server. See docs/dev.md.
package apitest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLImage is pinned so parallel runs on one machine share the pulled image.
const MySQLImage = "mysql:8.4"

// ContainerLabel marks every container the harness starts, for cleanup:
//
//	docker rm -f $(docker ps -aq --filter label=xproj-test)
const ContainerLabel = "xproj-test"

// EnvDSN lets a developer reuse an existing MySQL 8 server instead of a container.
// Value is a go-sql-driver DSN without a database name, e.g. "root:pw@tcp(127.0.0.1:3306)/".
const EnvDSN = "XP_TEST_MYSQL_DSN"

var serverDSN string // root DSN of the server, no database selected

// Main runs the package's tests against a MySQL server: either EnvDSN, or a
// throwaway container with a unique name and a dynamic host port. Call it from TestMain.
func Main(m *testing.M) {
	code := func() int {
		if dsn := os.Getenv(EnvDSN); dsn != "" {
			serverDSN = dsn
		} else {
			dsn, stop, err := startContainer()
			if err != nil {
				fmt.Fprintln(os.Stderr, "apitest: start MySQL:", err)
				return 1
			}
			defer stop()
			serverDSN = dsn
		}
		return m.Run()
	}()
	os.Exit(code)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func startContainer() (dsn string, stop func(), err error) {
	name := "xproj-test-" + randomHex(4)
	const pw = "test"
	out, err := exec.Command("docker", "run", "-d", "--rm",
		"--name", name, "--label", ContainerLabel+"=1",
		"-e", "MYSQL_ROOT_PASSWORD="+pw,
		"--tmpfs", "/var/lib/mysql",
		"-p", "127.0.0.1::3306",
		MySQLImage).CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("docker run: %v: %s", err, out)
	}
	stop = func() { _ = exec.Command("docker", "rm", "-f", name).Run() }

	out, err = exec.Command("docker", "port", name, "3306/tcp").Output()
	if err != nil {
		stop()
		return "", nil, fmt.Errorf("docker port: %v", err)
	}
	hostPort := strings.TrimSpace(strings.Split(strings.TrimSpace(string(out)), "\n")[0])
	dsn = fmt.Sprintf("root:%s@tcp(%s)/", pw, hostPort)

	// Probe over TCP on the mapped port: the image's init phase runs a temporary
	// server without networking, so only the real server answers here.
	deadline := time.Now().Add(3 * time.Minute)
	for {
		db, _ := sql.Open("mysql", dsn)
		err = db.Ping()
		_ = db.Close()
		if err == nil {
			return dsn, stop, nil
		}
		if time.Now().After(deadline) {
			stop()
			return "", nil, fmt.Errorf("MySQL not ready: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// freshDatabase creates an empty database for one test and drops it afterwards.
// It returns a DSN that selects it.
func freshDatabase(t testing.TB) string {
	t.Helper()
	if serverDSN == "" {
		t.Fatal("apitest: no MySQL server; call apitest.Main from TestMain")
	}
	name := "xp_" + randomHex(6)
	root, err := sql.Open("mysql", serverDSN)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = root.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		_ = root.Close()
	})
	return strings.TrimSuffix(serverDSN, "/") + "/" + name
}
