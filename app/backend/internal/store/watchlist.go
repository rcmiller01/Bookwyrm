package store

import (
	"errors"
	"sync"

	"app-backend/internal/domain"
)

var ErrWatchlistNotFound = errors.New("watchlist item not found")

type WatchlistStore interface {
	List(userID string) []domain.WatchlistItem
	Create(item domain.WatchlistItem) domain.WatchlistItem
	Delete(userID string, id string) error
}

type inMemoryWatchlistStore struct {
	mu    sync.RWMutex
	items map[string][]domain.WatchlistItem
}

func NewInMemoryWatchlistStore() WatchlistStore {
	return &inMemoryWatchlistStore{items: map[string][]domain.WatchlistItem{}}
}

func (s *inMemoryWatchlistStore) List(userID string) []domain.WatchlistItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.items[userID]
	cp := make([]domain.WatchlistItem, len(items))
	copy(cp, items)
	return cp
}

func (s *inMemoryWatchlistStore) Create(item domain.WatchlistItem) domain.WatchlistItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.UserID] = append(s.items[item.UserID], item)
	return item
}

func (s *inMemoryWatchlistStore) Delete(userID string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.items[userID]
	for i := range items {
		if items[i].ID == id {
			s.items[userID] = append(items[:i], items[i+1:]...)
			return nil
		}
	}
	return ErrWatchlistNotFound
}
