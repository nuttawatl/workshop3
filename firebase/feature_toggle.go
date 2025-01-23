package firebase

type featureToggle struct {
	EnableScheduleMonthly bool
	EnableScheduleOnce    bool
	TransferLimit         string
}

func Features(rc RemoteConfig) featureToggle {
	var ft featureToggle
	for key, value := range rc.Parameters {
		switch key {
		case "enable_schedule_monthly":
			ft.EnableScheduleMonthly = value.DefaultValue.Value == "true"
		case "enable_schedule_once":
			ft.EnableScheduleOnce = value.DefaultValue.Value == "true"
		case "transfer_limit":
			ft.TransferLimit = value.DefaultValue.Value
		}
	}
	return ft
}
