package application

import "sort"

type Asset struct{ Symbol string; DailyVolUSD float64 }

type Mesh struct{ Hot, Warm, Cold []Asset }

func PartitionMesh(assets []Asset) Mesh {
	sort.Slice(assets, func(i,j int) bool { return assets[i].DailyVolUSD > assets[j].DailyVolUSD })
	hot := assets
	if len(hot) > 30 { hot = hot[:30] }
	warmStart := len(hot)
	warmEnd := min(len(assets), warmStart+70)
	warm := assets[warmStart:warmEnd]
	cold := assets[warmEnd:]
	return Mesh{ Hot: hot, Warm: warm, Cold: cold }
}

func min(a,b int) int { if a<b {return a}; return b }
