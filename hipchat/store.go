package hipchat

type Store interface {
	SaveCredentials(i *InstallRecord) error
	DeleteCredentials(oAuthID string) error
	GetCredentials(groupID uint64) (*InstallRecord, error)
	GetGroupID(roomID uint32) (uint32, error) // temporary
}
