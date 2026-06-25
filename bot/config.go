package bot

const (
	GuildID        = "1387568839285280812"
	RoleLogsBindID = "1469869888934907966"

	RoleDevID     = "1387854054390108190" // 开发者
	RoleSponsorID = "1387660171156783144" // 赞助者

	// DevChannelID — #开发, where webhook-driven CI/CD + GitHub events land.
	// Channel IDs aren't secret (they don't grant access), keeping them in
	// code keeps the deploy/event channel obvious without spelunking k8s.
	DevChannelID = "1387790952047054939"
)
