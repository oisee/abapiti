package rewrite

func init() {
	RuntimeEffects["clock.epochMilliseconds"] = OpEffect{Reads: OpReads{Global: true}, Writes: "None", MayRaise: "None", Aliases: "None"}
	RuntimeEffects["weak.lookupKey"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "None", Aliases: "Receiver"}
	RuntimeEffects["weak.writeKey"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "Catchable", Aliases: "Receiver"}
}
