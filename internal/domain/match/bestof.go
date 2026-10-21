package match

// Allowed 是支持的赛制。
var Allowed = []int{1, 3, 5}

// IsValid 报告赛制是否合法。
func IsValid(bestOf int) bool {
	for _, v := range Allowed {
		if v == bestOf {
			return true
		}
	}
	return false
}

// WinsRequired 返回系列赛所需胜场：BO1=1、BO3=2、BO5=3。
func WinsRequired(bestOf int) int { return bestOf/2 + 1 }

// Winner 按蓝/红胜场判定系列赛胜方。
func Winner(bestOf, blueWins, redWins int) (side string, ok bool) {
	need := WinsRequired(bestOf)
	if blueWins >= need {
		return "blue", true
	}
	if redWins >= need {
		return "red", true
	}
	return "", false
}

// IsOver 报告系列赛是否已分出胜负。
func IsOver(bestOf, blueWins, redWins int) bool {
	_, ok := Winner(bestOf, blueWins, redWins)
	return ok
}
