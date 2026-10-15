// Package corpusabap embeds the A4H registry runtime for standalone binaries.
package corpusabap

import "embed"

// Runtime contains the corpus/log classes, their interface and tables, and the
// reports used to load corpus data and configure the application log. The RFC
// upload function group and its list types are not dependencies of this runtime.
//
//go:embed src/zcl_abapiti_corpus.clas.* src/zcl_abapiti_log.clas.* src/zif_abapiti_corpus.intf.* src/zabapiti_corpus.tabl.xml src/zabapiti_runlog.tabl.xml src/zabapiti_corpus_load.prog.* src/zabapiti_log_setup.prog.*
var Runtime embed.FS
