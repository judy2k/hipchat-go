package hipchat

type Store interface {
	SaveCredentials(i *InstallRecord) error
	DeleteCredentials(oAuthID string) error
	GetCredentials(groupID, roomID uint32) (*InstallRecord, error)
	GetGroupID(roomID uint32) (uint32, error) // temporary
	GetOAuthSecret(oauthID string) (string, error) // Also temporary?
}
