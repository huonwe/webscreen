package sagent

func ConfigDescription() []ConfigParamDescription {

	return []ConfigParamDescription{
		{
			Name:        "av_sync",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "synchronize video and audio streams based on their PTS. Useful when driver provides reliable PTS.",
		},
		{
			Name:        "use_local_timestamp",
			Type:        "boolean",
			Required:    true,
			Default:     true,
			Description: "use local timestamp as PTS for AV sync instead of driver-provided PTS (useful when driver does not provide reliable PTS)",
		},
	}
}
