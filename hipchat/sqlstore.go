package hipchat

import (
	"database/sql"
	"log"
)

// SqlStore encapsulates a data store
type SqlStore struct {
	db *sql.DB
}

// NewSqlStore creates a new data store backed by a database.
func NewSqlStore(driverName string, dataSourceName string) (Store, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return &SqlStore{db}, nil
}

func (s *SqlStore) GetGroupID(roomID uint32) (uint32, error) {
	var result uint32
	log.Printf("Looking up group-id for room-id: %v", roomID)
	err := s.db.QueryRow(
		"SELECT groupid from installation where roomid = $1", roomID).Scan(
		&result)
	log.Printf("Result: %v", result)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	default:
		return result, nil
	}
}

// SaveCredentials saves a group's credentials to the SqlStore
func (s *SqlStore) SaveCredentials(i *InstallRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO installation (
            capabilitiesUrl, oauthId, oauthSecret, groupId, roomId
        ) VALUES (
            $1, $2, $3, $4, $5
        )`,
		i.CapabilitiesURL, i.OAuthID, i.OAuthSecret, i.GroupID, i.RoomID)
	return err
}

// DeleteCredentials removes the specified credentials from the database.
func (s *SqlStore) DeleteCredentials(oAuthID string) error {
	_, err := s.db.Exec(`DELETE FROM installation WHERE oauthId = $1`, oAuthID)
	return err
}

// GetCredentials obtains a group's credentials from the SqlStore
func (s *SqlStore) GetCredentials(groupID, roomID uint32) (*InstallRecord, error) {
	c := &InstallRecord{}
	err := s.db.QueryRow(
		"SELECT capabilitiesUrl, oauthId, oauthSecret, groupId, roomId FROM installation WHERE groupId = $1 AND roomId = $2", groupID, roomID).Scan(
		&c.CapabilitiesURL, &c.OAuthID, &c.OAuthSecret, &c.GroupID, &c.RoomID)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return c, nil
	}
}
