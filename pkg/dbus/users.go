package dbus

import (
	"bufio"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// User represents a local account eligible for graphical login.
type User struct {
	Username string
	UID      int
	RealName string
	HomeDir  string
	Shell    string
}

// minLoginUID is the minimum UID considered a human (login-eligible) account.
// System/daemon accounts (UID < 1000) are excluded from the greeter list.
const minLoginUID = 1000

// shellBlocklist lists shells that must never appear in the greeter user
// list. Accounts with a non-login shell cannot start a session.
var shellBlocklist = map[string]bool{
	"/bin/false":        true,
	"/usr/bin/false":    true,
	"/sbin/nologin":     true,
	"/usr/sbin/nologin": true,
	"/bin/nologin":      true,
	"/usr/bin/nologin":  true,
	"false":             true,
	"nologin":           true,
}

// ListUsers enumerates local accounts suitable for the greeter user
// selection list by parsing /etc/passwd. Accounts with UID < minLoginUID or
// a non-login shell are excluded; the result is sorted by username.
func ListUsers() []User {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil
	}
	defer f.Close()
	return parsePasswd(f, minLoginUID)
}

// parsePasswd parses a passwd-format stream and returns login-eligible
// users (UID >= minUID, non-blocklisted shell), sorted by username.
func parsePasswd(r io.Reader, minUID int) []User {
	var users []User
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}
		uid, err := strconv.Atoi(fields[2])
		if err != nil || uid < minUID {
			continue
		}
		shell := fields[6]
		if shell == "" || shellBlocklist[shell] {
			continue
		}
		users = append(users, User{
			Username: fields[0],
			UID:      uid,
			RealName: fields[4],
			HomeDir:  fields[5],
			Shell:    shell,
		})
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users
}
