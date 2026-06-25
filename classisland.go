package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ClassIslandProfile 解析 ClassIsland 的 Profile.json
type ClassIslandProfile struct {
	ClassPlans []ClassIslandClassPlan `json:"ClassPlans"`
}

// ClassIslandClassPlan 课程计划
type ClassIslandClassPlan struct {
	Name      string                   `json:"Name"`
	TimeTable ClassIslandTimeTable     `json:"TimeTable"`
}

// ClassIslandTimeTable 时间表
type ClassIslandTimeTable struct {
	Layouts []ClassIslandTimeLayout `json:"Layouts"`
}

// ClassIslandTimeLayout 时间段布局
type ClassIslandTimeLayout struct {
	StartTime ClassIslandTimePoint `json:"StartTime"`
	EndTime   ClassIslandTimePoint `json:"EndTime"`
	Subject   string               `json:"Subject"`
}

// ClassIslandTimePoint 时间点（H, M, S）
type ClassIslandTimePoint struct {
	H int `json:"H"`
	M int `json:"M"`
	S int `json:"S"`
}

// ClassIslandState 当前课程状态
type ClassIslandState struct {
	Connected     bool   `json:"connected"`
	CurrentClass  string `json:"current_class"`
	NextClass     string `json:"next_class"`
	ProfilePath   string `json:"profile_path"`
	Error         string `json:"error,omitempty"`
}

// findClassIslandProfile 自动查找 ClassIsland 的 Profile.json
func findClassIslandProfile() string {
	// 常见路径
	paths := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "ClassIsland", "Profiles", "Profile.json"),
		filepath.Join(os.Getenv("APPDATA"), "ClassIsland", "Profiles", "Profile.json"),
		"ClassIsland", // 相对路径
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// getClassIslandState 读取 ClassIsland 当前课程状态
// configPath: 用户配置中指定的路径，为空则自动查找
func getClassIslandState(configPath string) ClassIslandState {
	state := ClassIslandState{}

	profilePath := configPath
	if profilePath == "" {
		profilePath = findClassIslandProfile()
	}
	if profilePath == "" {
		state.Error = "未找到 ClassIsland 配置文件"
		return state
	}

	state.Connected = true
	state.ProfilePath = profilePath

	data, err := os.ReadFile(profilePath)
	if err != nil {
		state.Error = fmt.Sprintf("读取 Profile 失败: %v", err)
		return state
	}

	var profile ClassIslandProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		state.Error = fmt.Sprintf("解析 Profile 失败: %v", err)
		return state
	}

	if len(profile.ClassPlans) == 0 {
		state.Error = "Profile 中没有课程计划"
		return state
	}

	// 获取今天的课程计划（取第一个计划，ClassIsland 通常只有一个活跃计划）
	plan := profile.ClassPlans[0]
	now := time.Now()
	currentTime := ClassIslandTimePoint{H: now.Hour(), M: now.Minute(), S: now.Second()}

	for i, layout := range plan.TimeTable.Layouts {
		if isTimeInRange(currentTime, layout.StartTime, layout.EndTime) {
			state.CurrentClass = layout.Subject
			// 查找下一节课
			if i+1 < len(plan.TimeTable.Layouts) {
				state.NextClass = plan.TimeTable.Layouts[i+1].Subject
			}
			return state
		}
		// 查找下一节课（当前时间在课程之前）
		if isTimeBefore(currentTime, layout.StartTime) {
			state.NextClass = layout.Subject
			return state
		}
	}

	return state
}

// isTimeInRange 判断当前时间是否在 [start, end] 范围内
func isTimeInRange(now, start, end ClassIslandTimePoint) bool {
	nowSec := now.H*3600 + now.M*60 + now.S
	startSec := start.H*3600 + start.M*60 + start.S
	endSec := end.H*3600 + end.M*60 + end.S
	return nowSec >= startSec && nowSec <= endSec
}

// isTimeBefore 判断 now 是否在 target 之前
func isTimeBefore(now, target ClassIslandTimePoint) bool {
	nowSec := now.H*3600 + now.M*60 + now.S
	targetSec := target.H*3600 + target.M*60 + target.S
	return nowSec < targetSec
}