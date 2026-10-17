package rewrite

func init() {
	RuntimeEffects["set.iterationValues"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "Receiver"}
	RuntimeEffects["clock.epochMilliseconds"] = OpEffect{Reads: OpReads{Global: true}, Writes: "None", MayRaise: "None", Aliases: "None"}
	RuntimeEffects["weak.lookupKey"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "None", Aliases: "Receiver"}
	RuntimeEffects["map.insertions"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "None", Aliases: "None"}
	RuntimeEffects["weak.writeKey"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "Catchable", Aliases: "Receiver"}
}
