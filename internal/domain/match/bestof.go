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

// IsOver 报告系列赛是否已分出胜负（参数为 1队/2队 胜场，不是蓝/红方）。
func IsOver(bestOf, team1Wins, team2Wins int) bool {
	need := WinsRequired(bestOf)
	return team1Wins >= need || team2Wins >= need
}
