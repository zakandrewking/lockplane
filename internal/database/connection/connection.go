package connection

type ConnectionConfig struct {
	DatabaseType string // TODO make enum?
	PostgresUrl  string
}
