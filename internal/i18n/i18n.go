package i18n

type Text struct {
	AppTitle            string
	Protected           string
	NotInstalled        string
	RepairNeeded        string
	Checking            string
	RepairAll           string
	LaunchStable        string
	LaunchPTB           string
	LaunchCanary        string
	StartWithWindows    string
	OpenLogs            string
	Exit                string
	RepairSuccessTitle  string
	RepairSuccessBody   string
	RepairFailedTitle   string
	RepairFailedBody    string
	DiscordNotInstalled string
	AlreadyRunning      string
}

func ForTraditionalChinese(chinese bool) Text {
	if chinese {
		return Text{
			AppTitle:            "VencordGuard",
			Protected:           "已保護",
			NotInstalled:        "未安裝",
			RepairNeeded:        "需要修復",
			Checking:            "檢查中",
			RepairAll:           "全部修復",
			LaunchStable:        "啟動 Discord Stable",
			LaunchPTB:           "啟動 Discord PTB",
			LaunchCanary:        "啟動 Discord Canary",
			StartWithWindows:    "隨 Windows 啟動",
			OpenLogs:            "開啟紀錄資料夾",
			Exit:                "結束",
			RepairSuccessTitle:  "Vencord 已修復",
			RepairSuccessBody:   "%s 已修復並可正常使用。",
			RepairFailedTitle:   "Vencord 修復失敗",
			RepairFailedBody:    "無法修復 %s：%v\n\n請查看 VencordGuard 紀錄。",
			DiscordNotInstalled: "%s 尚未安裝。",
			AlreadyRunning:      "VencordGuard 已在背景執行。",
		}
	}
	return Text{
		AppTitle:            "VencordGuard",
		Protected:           "Protected",
		NotInstalled:        "Not installed",
		RepairNeeded:        "Repair needed",
		Checking:            "Checking",
		RepairAll:           "Repair all",
		LaunchStable:        "Launch Discord Stable",
		LaunchPTB:           "Launch Discord PTB",
		LaunchCanary:        "Launch Discord Canary",
		StartWithWindows:    "Start with Windows",
		OpenLogs:            "Open logs folder",
		Exit:                "Exit",
		RepairSuccessTitle:  "Vencord repaired",
		RepairSuccessBody:   "%s was repaired and is ready to use.",
		RepairFailedTitle:   "Vencord repair failed",
		RepairFailedBody:    "Could not repair %s: %v\n\nSee the VencordGuard log for details.",
		DiscordNotInstalled: "%s is not installed.",
		AlreadyRunning:      "VencordGuard is already running in the background.",
	}
}
