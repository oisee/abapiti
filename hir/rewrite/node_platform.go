package rewrite

func init() {
	RuntimeEffects["string.sha256Hex"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Fresh UTF-8 SHA-256 hex pipeline; Go is pure, ABAP explicitly refuses."}
	RuntimeEffects["path.basename"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "POSIX basename; Go is pure, ABAP explicitly refuses."}
	RuntimeEffects["path.join"] = OpEffect{Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None", Conservative: "POSIX join/normalize of the segment array; Go is pure, ABAP explicitly refuses."}
	RuntimeEffects["util.isDeepStrictEqual"] = OpEffect{Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Reads both boxed graphs; uncertified kinds and cycles trap; ABAP refuses."}
	for _, op := range []string{"fs.readText", "fs.readdirNames", "fs.readdirEntries"} {
		RuntimeEffects[op] = OpEffect{Reads: OpReads{Receiver: true, Global: true}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Host filesystem read; failures throw Error."}
	}
}
