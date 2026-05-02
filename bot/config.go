package bot

const (
	GuildID        = "1387568839285280812"
	RoleLogsBindID = "1469869888934907966"

	// DevChannelID — #开发, where webhook-driven CI/CD + GitHub events land.
	// Channel IDs aren't secret (they don't grant access), keeping them in
	// code keeps the deploy/event channel obvious without spelunking k8s.
	DevChannelID = "1387790952047054939"
)
