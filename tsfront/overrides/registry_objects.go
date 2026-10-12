package overrides

import "github.com/oisee/abapiti/hir"

// Object parse clocks measure durations only; they never expose epoch time.
// Keep these adaptations pinned to the reviewed consumers rather than changing Date.
func registryObjectParsers() []Entry {
	return []Entry{
		{FullSource: true, ID: "abaplint-object-functiongroup-text-xml", Key: Key{"src/objects/function_group.ts", "FunctionGroup.getTextSymbols", "KindMethodDeclaration"}, SHA256: "4815636396a262741c4c95ca5c916c1b1c7398608d50a68f8ed9ddfd7d6997b9", Rationale: "same reviewed XML options as AbstractObject.parseRaw2; keep the source catch and text extraction", Expressions: map[string]func() *hir.Expr{
			`new XMLParser({parseTagValue: false, ignoreAttributes: true, trimValues: false}).parse(found.getRaw())`: objectTextXML,
		}},
		{ID: "abaplint-object-authorizationcheckfield-parse", Key: Key{"src/objects/authorization_check_field.ts", "AuthorizationCheckField.parse", "KindMethodDeclaration"}, SHA256: "ba1b879963053ff6946002a80ca0004c3debf1a28a68d15291a7e26494f97912", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-cdsmetadataextension-parse", Key: Key{"src/objects/cds_metadata_extension.ts", "CDSMetadataExtension.parse", "KindMethodDeclaration"}, SHA256: "1df3243fc8495a38af349c171e76315f01c67192ee18bf951d7c84b551b0234e", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-datadefinition-parse", Key: Key{"src/objects/data_definition.ts", "DataDefinition.parse", "KindMethodDeclaration"}, SHA256: "1a8ae95b6b0b33a6d184ab604a050fd66c72bb1b635bbaf4d022cf522e8921cb", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-dataelement-parse", Key: Key{"src/objects/data_element.ts", "DataElement.parse", "KindMethodDeclaration"}, SHA256: "dfdf151c4fbe3a52d07338ebcb19e86c6812ac88961945fff4e0d7a865d7bff1", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock, "JSON.parse(jsonFile.getRaw())": objectAFFJSON}},
		{ID: "abaplint-object-domain-parse", Key: Key{"src/objects/domain.ts", "Domain.parse", "KindMethodDeclaration"}, SHA256: "db1156ba28a9a58f466daaff9cdc1f059c3706ffacee80a88d476e3f97d5d2f5", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock, "JSON.parse(jsonFile.getRaw())": objectAFFJSON}},
		{ID: "abaplint-object-enhancementimplementation-parse", Key: Key{"src/objects/enhancement_implementation.ts", "EnhancementImplementation.parse", "KindMethodDeclaration"}, SHA256: "818e37b05fb7cc3efb16e5a17db274886c1deeb531480d1570c0bb8c8b0f3e9d", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-extensionindex-parse", Key: Key{"src/objects/extension_index.ts", "ExtensionIndex.parse", "KindMethodDeclaration"}, SHA256: "4a87c6512ba5a9e25de15daa1aba874c981416c720337022402fce501ba62245", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-icfservice-parse", Key: Key{"src/objects/icf_service.ts", "ICFService.parse", "KindMethodDeclaration"}, SHA256: "8802e131661df0f904797f9ff3980c87ddd34a5aba90bbbaa229ed21de7e6ab4", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-lockobject-parse", Key: Key{"src/objects/lock_object.ts", "LockObject.parse", "KindMethodDeclaration"}, SHA256: "72419cd08928cd5a8a239f272e42fdf5a92cd40134fee292bb8413c00c55575a", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-maintenanceandtransportobject-parse", Key: Key{"src/objects/maintenance_and_transport_object.ts", "MaintenanceAndTransportObject.parse", "KindMethodDeclaration"}, SHA256: "645ebf32a4c6c3440ea8a6fa013e200f003fe295f0129db6898f14a7dc14ee64", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-mimeobject-parse", Key: Key{"src/objects/mime_object.ts", "MIMEObject.parse", "KindMethodDeclaration"}, SHA256: "47ee40faec7c114ae83ba507fd1a46a51555c351980048c50d156cab4589966c", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-numberrange-parse", Key: Key{"src/objects/number_range.ts", "NumberRange.parse", "KindMethodDeclaration"}, SHA256: "6727a90bd130162651cf40375f375b69cb994b5468bd57d7fbf41d91ce03f77e", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-proxyobject-parse", Key: Key{"src/objects/proxy_object.ts", "ProxyObject.parse", "KindMethodDeclaration"}, SHA256: "983ba3480925bf842654f8897c5217e6c9a4a00807add863431de73e364b7032", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-transaction-parse", Key: Key{"src/objects/transaction.ts", "Transaction.parse", "KindMethodDeclaration"}, SHA256: "287865fcd4c40007854f4dd0fe2ce9f7a4edc5c5782d81ad096504915b5c7612", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-object-webmime-parse", Key: Key{"src/objects/web_mime.ts", "WebMIME.parse", "KindMethodDeclaration"}, SHA256: "624635fee4e92558fd25c99162f88a664939f89a64399bdd420fde09a3b366ae", Rationale: "parse duration is telemetry only; AFF uses the strict JSON graph adapter", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
	}
}

func objectTextXML() *hir.Expr {
	typ := hir.Type{Kind: hir.InterfaceRef, Name: "src/files/_ifile.ts.IFile"}
	file := &hir.Expr{Kind: hir.Narrow, Type: typ, X: hir.V("found", hir.T(hir.Optional, typ))}
	raw := &hir.Expr{Kind: hir.VirtualCall, Name: "getRaw", Type: hir.T(hir.String), X: file}
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "xml.parseSubset", Type: hir.T(hir.Dynamic), X: raw}
}

func objectAFFJSON() *hir.Expr {
	typ := hir.Type{Kind: hir.InterfaceRef, Name: "src/files/_ifile.ts.IFile"}
	file := &hir.Expr{Kind: hir.Narrow, Type: typ, X: hir.V("jsonFile", hir.T(hir.Optional, typ))}
	raw := &hir.Expr{Kind: hir.VirtualCall, Name: "getRaw", Type: hir.T(hir.String), X: file}
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "json.parseSubset", Type: hir.T(hir.Dynamic), X: raw}
}
