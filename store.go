package counterapi

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Store keeps the counters in one bbolt file: a bucket per counter, each
// step's ordinal as the key and the step's JSON as the value. The file is
// the volume — it survives restarts, a snapshot copies it byte for
// byte, and restoring opens the snapshot as a store of its own.
// orc:ADR-0002#decision.c9
type Store struct {
	db *bolt.DB
}

// Open opens the store at path, creating the file when it is absent.
// orc:ADR-0002#decision.c9
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the store; a later Open of the same file answers with the
// same steps.
// orc:ADR-0002#decision.c9
func (s *Store) Close() error { return s.db.Close() }

// Append records one step on the counter: the next ordinal in the
// counter's own sequence, at time t, and answers with the step it
// recorded. A counter that did not exist starts at ordinal one.
// orc:ADR-0002#decision.c1
func (s *Store) Append(id string, t time.Time) (Step, error) {
	var step Step
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(id))
		if err != nil {
			return err
		}
		n, err := b.NextSequence()
		if err != nil {
			return err
		}
		step = newStep(n, t)
		v, err := step.json()
		if err != nil {
			return err
		}
		return b.Put(ordinalKey(n), v)
	})
	return step, err
}

// Steps answers the counter's recorded steps in the order they were
// recorded, and whether the counter exists at all.
// orc:ADR-0002#decision.c3
func (s *Store) Steps(id string) ([]Step, bool, error) {
	var steps []Step
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return nil
		}
		found = true
		return b.ForEach(func(k, v []byte) error {
			step, err := stepFromJSON(v)
			if err != nil {
				return fmt.Errorf("reading step %s of counter %q: %w", k, id, err)
			}
			steps = append(steps, step)
			return nil
		})
	})
	return steps, found, err
}

// Tally answers how many steps the counter has recorded, and whether the
// counter exists at all.
// orc:ADR-0002#decision.c4
func (s *Store) Tally(id string) (int, bool, error) {
	count := 0
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return nil
		}
		found = true
		return b.ForEach(func(_, _ []byte) error {
			count++
			return nil
		})
	})
	return count, found, err
}

// Reset removes the counter with every recorded step, and answers whether
// the counter existed. The next append to the same id starts a fresh
// counter at ordinal one.
// orc:ADR-0002#decision.c5
func (s *Store) Reset(id string) (bool, error) {
	found := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte(id)) == nil {
			return nil
		}
		found = true
		return tx.DeleteBucket([]byte(id))
	})
	return found, err
}

// Ping reads the store: it answers nil while the store answers at all.
// A readiness probe rides this, so liveness stays independent of it.
// orc:ADR-0002#decision.c7
func (s *Store) Ping() error {
	return s.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(_ []byte, _ *bolt.Bucket) error { return nil })
	})
}

// SaveSnapshot writes the store's byte-exact copy to w.
// orc:ADR-0002#decision.c9
func (s *Store) SaveSnapshot(w io.Writer) error {
	return s.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(w)
		return err
	})
}

// RestoreFrom opens a new store from a snapshot: the snapshot's counters
// answer every later read. The snapshot lands as a file in dir; the
// store keeps it open.
// orc:ADR-0002#decision.c9
func RestoreFrom(r io.Reader, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "snapshot.db")
	if err := writeAll(path, r); err != nil {
		return nil, err
	}
	return Open(path)
}

// ordinalKey is the step's ordered place in the file: the ordinal as
// eight big-endian bytes, so the file's byte order is the recording
// order.
// orc:ADR-0002#decision.c2
func ordinalKey(n uint64) []byte {
	var k [8]byte
	binary.BigEndian.PutUint64(k[:], n)
	return k[:]
}

// writeAll writes r to the file at path, one durable write.
func writeAll(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}
