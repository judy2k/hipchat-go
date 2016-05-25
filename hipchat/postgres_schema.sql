DROP SEQUENCE IF EXISTS serial CASCADE;
CREATE SEQUENCE serial;

DROP TABLE IF EXISTS installation CASCADE;
CREATE TABLE installation (
    oauthId varchar(255) PRIMARY KEY,
    capabilitiesUrl varchar(255) NOT NULL,
    oauthSecret varchar(255) NOT NULL,
    groupId integer NOT NULL,
    roomId integer
);

DROP INDEX IF EXISTS installation_uniq CASCADE;
CREATE UNIQUE INDEX installation_uniq ON installation (
    groupId, roomId
);