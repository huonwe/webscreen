package sagent

func ConfigDescription() []ConfigParamDescription {

	return []ConfigParamDescription{
		{
			Name:        "av_sync",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "Enable A/V sync. Useful when watching videos.",
		},
		{
			Name:        "use_local_timestamp",
			Type:        "boolean",
			Required:    true,
			Default:     true,
			Description: "Use local timestamp instead of device timestamp. This may reduce latency but the video may be less smooth.",
		},
	}
}
