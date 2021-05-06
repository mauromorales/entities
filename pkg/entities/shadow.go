// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package entities

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	permbits "github.com/phayes/permbits"
	"github.com/pkg/errors"
)

// ParseShadow opens the file and parses it into a map from usernames to Entries
func ParseShadow(path string) (map[string]Shadow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return ParseReader(file)
}

// ParseReader consumes the contents of r and parses it into a map from
// usernames to Entries
func ParseReader(r io.Reader) (map[string]Shadow, error) {
	lines := bufio.NewReader(r)
	entries := make(map[string]Shadow)
	for {
		line, _, err := lines.ReadLine()
		if err != nil {
			break
		}
		name, entry, err := parseLine(string(copyBytes(line)))
		if err != nil {
			return nil, err
		}
		entries[name] = entry
	}
	return entries, nil
}

func parseLine(line string) (string, Shadow, error) {
	fs := strings.Split(line, ":")
	if len(fs) != 9 {
		return "", Shadow{}, errors.New("Unexpected number of fields in /etc/shadow: found " + strconv.Itoa(len(fs)))
	}

	return fs[0], Shadow{fs[0], fs[1], fs[2], fs[3], fs[4], fs[5], fs[6], fs[7], fs[8]}, nil
}

func copyBytes(x []byte) []byte {
	y := make([]byte, len(x))
	copy(y, x)
	return y
}

type Shadow struct {
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	LastChanged    string `yaml:"last_changed"`
	MinimumChanged string `yaml:"minimum_changed"`
	MaximumChanged string `yaml:"maximum_changed"`
	Warn           string `yaml:"warn"`
	Inactive       string `yaml:"inactive"`
	Expire         string `yaml:"expire"`
	Reserved       string `yaml:"reserved"`
}

func (u Shadow) String() string {
	return strings.Join([]string{u.Username,
		u.Password,
		u.LastChanged,
		u.MinimumChanged,
		u.MaximumChanged,
		u.Warn,
		u.Inactive,
		u.Expire,
		u.Reserved,
	}, ":")
}

func shadowDefault(s string) string {
	if s == "" {
		s = "/etc/shadow"
	}
	return s
}

func (u Shadow) prepare() Shadow {
	if u.LastChanged == "now" {
		// POST: Set in last_changed the current days from 1970
		now := time.Now()
		days := now.Unix() / 24 / 60 / 60
		u.LastChanged = fmt.Sprintf("%d", days)
	}
	return u
}

// FIXME: Delete can be shared across all of the supported Entities
func (u Shadow) Delete(s string) error {
	s = shadowDefault(s)
	input, err := ioutil.ReadFile(s)
	if err != nil {
		return errors.Wrap(err, "Could not read input file")
	}
	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}
	lines := bytes.Replace(input, []byte(u.String()+"\n"), []byte(""), 1)

	err = ioutil.WriteFile(s, []byte(lines), os.FileMode(permissions))
	if err != nil {
		return errors.Wrap(err, "Could not write")
	}

	return nil
}

// FIXME: Create can be shared across all of the supported Entities
func (u Shadow) Create(s string) error {
	s = shadowDefault(s)

	u = u.prepare()
	current, err := ParseShadow(s)
	if err != nil {
		return errors.Wrap(err, "Failed parsing passwd")
	}
	if _, ok := current[u.Username]; ok {
		return errors.New("Entity already present")
	}
	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}
	f, err := os.OpenFile(s, os.O_APPEND|os.O_WRONLY, os.FileMode(permissions))
	if err != nil {
		return errors.Wrap(err, "Could not read")
	}

	defer f.Close()

	if _, err = f.WriteString(u.String() + "\n"); err != nil {
		return errors.Wrap(err, "Could not write")
	}
	return nil
}

func (u Shadow) Apply(s string) error {
	s = shadowDefault(s)

	u = u.prepare()
	current, err := ParseShadow(s)
	if err != nil {
		return errors.Wrap(err, "Failed parsing passwd")
	}
	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}

	if _, ok := current[u.Username]; ok {
		input, err := ioutil.ReadFile(s)
		if err != nil {
			return errors.Wrap(err, "Could not read input file")
		}

		lines := strings.Split(string(input), "\n")

		for i, line := range lines {
			if entityIdentifier(line) == u.Username {
				lines[i] = u.String()
			}
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(s, []byte(output), os.FileMode(permissions))
		if err != nil {
			return errors.Wrap(err, "Could not write")
		}

	} else {
		// Add it
		return u.Create(s)
	}

	return nil
}
