# __navi__ · `cmd/multisend-agent/` — 16 files → symbols at exact line numbers
<!-- navindex · 2026-08-18 · DO NOT EDIT BY HAND; regen via navindex skill -->
↑ repo tree: [`../../__navi__.md`](../../__navi__.md)

- **config_api.go** (332 ln)
  <sub>L33:type editableConfig  L57:type configRuntimeView  L67:type configAPIResponse  L75:app.configHandler  L117:app.loadPersistedConfig  L133:configResponse  L151:editableConfigFrom  L177:applyEditableConfig  L268:normalizeSafeDirectory  L299:normalizeStringList  L324:oneOf</sub>
- **config_api_test.go** (163 ln)
  <sub>L16:writeConfigFixture  L25:TestConfigGetRedactsNodeSecret  L47:TestConfigPutValidatesAndPreservesRuntimeFields  L105:TestConfigPutRejectsUnsafeRootWithoutChangingFile  L125:TestConfigPutRejectsRelativeReceivePath  L139:TestConfigPutGeneratesSecretWhenAuthIsEnabled</sub>
- **doctor.go** (639 ln)
  <sub>L54:type doctorReporter  L61:doctorReporter.line  L64:doctorReporter.Pass  L65:doctorReporter.Warn  L66:doctorReporter.Fail  L67:doctorReporter.Skip  L69:RunDoctor  L93:type runtimeState  L100:readRuntimeState  L115:printDoctorHeader  L138:printDoctorPaths  L168:printInstalledFiles  L188:validateAndPrintConfig  L259:redactDiagnosticSecrets  L283:printAgentProcess  L321:testLocalAPI  L352:printFirewallChecks  L399:firewallRuleVisibleViaNetsh  L404:printAutostart  L425:printExplorerContext  L475:printInterfaces  L493:printSummary  L509:readConfigForDoctor  L532:fileVersion …</sub>
- **doctor_test.go** (26 ln)
  <sub>L5:TestRedactDiagnosticSecrets</sub>
- **download_smoke.go** (348 ln)
  <sub>L27:runDownloadSmoke  L277:toFloat  L290:toInt  L304:printDownloadFailedChunks  L314:printDownloadManifestErrors</sub>
- **history.go** (581 ln)
  <sub>L55:type persistedJobState  L69:type persistedPullState  L74:type operationHistoryFile  L79:app.restoreOperationHistory  L84:app.operationHistoryDir  L91:app.persistJobLocked  L118:app.persistPullLocked  L132:saveOperationHistory  L164:app.restoreJobHistory  L185:app.jobStateFromHistory  L256:app.validateRestoredJobOptions  L280:validP2PHistoryManifest  L321:sourceMatchesManifest  L330:manifestAllDone  L339:app.restorePullHistory  L377:recentOperationHistory  L399:validOperationID  L413:app.pruneJobsLocked  L429:app.prunePullsLocked  L445:app.forgetJob  L475:validateManagedCleanupPath  L512:removeManagedCleanupPath  L523:app.forgetPull  L548:samePath …</sub>
- **history_test.go** (286 ln)
  <sub>L23:TestOperationHistoryRestoresSafeJobAndPullContext  L91:TestOperationHistoryMarksUnsafeManifestNonResumable  L140:TestOperationHistoryRestoresCompletedJobWithoutSourceFile  L169:TestForgetOperationHistoryOnlyRemovesTerminalRecord  L201:TestForgetOperationHistoryRejectsActiveAndUnavailableStorage  L214:TestManagedLauncherCleanupPathAndForget  L256:writeP2PHistoryManifest</sub>
- **http_api.go** (159 ln)
  <sub>L16:app.localHandler  L46:app.shutdownAgent  L62:app.controlHandler  L69:newAgentHTTPServer  L81:localRequestGuard  L99:isLoopbackHost  L112:isSameRequestOrigin  L124:decodeJSONRequest  L128:decodeOptionalJSONRequest  L132:decodeJSONRequestAllowEmpty</sub>
- **http_api_test.go** (124 ln)
  <sub>L14:localRequest  L20:TestLocalHandlerRedirectsAndServesEmbeddedUI  L43:TestLocalRequestGuardRejectsExternalHostAndOrigin  L71:TestReadOnlyHandlersRejectPost  L91:TestJSONRequestBodyLimit  L100:TestShutdownAgentRespondsBeforeCancel  L116:TestNewAgentHTTPServerHasDefensiveTimeouts</sub>
- **lab_smoke.go** (531 ln)
  <sub>L36:type labRuntimeState  L41:type smokeJob  L66:type smokeResult  L72:smokeResult.passf  L80:smokeResult.warnf  L82:runLabSmoke  L362:readRuntime  L393:ensureDeterministicFile  L422:apiGET  L438:apiPOST  L455:canDial  L464:waitForSmokeReceiver  L488:printLabSummary  L501:printFailedChunkDetails</sub>
- **lab_smoke_test.go** (41 ln)
  <sub>L8:TestValidateLabReceivePath  L20:TestValidateLabReceivePathRejectOutside  L28:TestValidateLabReceivePathRejectEmpty  L35:TestValidateLabReceivePathRejectTraversal</sub>
- **main.go** (2852 ln)
  <sub>L143:type channelProgress  L157:type channelsProgress  L162:type jobDetails  L189:type receiverSession  L205:type failedChunkRef  L212:type progressSample  L219:type jobState  L239:type receiverSessionState  L246:type app  L261:type pullJobState  L279:type sendTargetOptions  L288:type apiErrorResponse  L298:type preparedRemoteSource  L309:agentLogPath  L320:setupAgentLogging  L342:rotateLogIfLarge  L355:main  L420:app.selectAndPersistPorts  L450:app.writeRuntimeState  L471:app.runReceiver  L494:app.handleConn  L624:app.updateReceiverManifest  L678:app.ensureReceiverSession  L692:app.updateReceiverSession …</sub>
- **main_test.go** (587 ln)
  <sub>L49:TestControlJSONClientsSendBearerToken  L74:TestControlJSONClientRejectsOversizedResponse  L87:TestControlAPITokenOnlyWhenAuthRequired  L98:TestControlAuthFailsClosedAndAcceptsCaseInsensitiveScheme  L115:TestRemoteSendPathAllowedResolvesSymlinkEscape  L143:TestStartSendRejectsInvalidChunkSizeBeforeCreatingJob  L157:TestSendRejectsLabReceivePathWhenLabFalse  L184:TestSendRejectsRelativePathAndInvalidChunkSize  L214:TestSendRejectsUnsafeCleanupPath  L228:TestPullRejectsInvalidChunkSizeBeforeDiscovery  L238:TestInterfacesHandlerPayload  L283:TestWriteAPIErrorReturnsJSON  L305:TestFormatRemoteAPIErrorIncludesRequestID  L315:TestMergeReceiverChunksOrdersByIndex  L344:TestValidateReceiverFinalSize  L358:TestCleanupReceiverChunksOnlyRemovesChunks  L386:TestFinalizeReceiverSessionIdempotent  L430:TestUpdateReceiverManifestSerializesConcurrentChunks  L476:TestValidateReceiverPathRejectsOutside  L484:TestParseFileSourceAcceptsUNCAndFileURL  L501:TestValidateOutputUnderReceiveReturnsRelativeDestination  L516:TestPrepareRemoteSendSourceFolderCreatesRelativeZip  L551:TestReceiverTargetInfoForPullPublishesOutsideSession  L565:TestExtractZipSafeRejectsTraversal</sub>
- **smoke_cleanup.go** (123 ln)
  <sub>L13:cleanupLabSmokeArtifacts  L27:cleanupDownloadSmokeArtifacts  L38:cleanupSmokeOperation  L65:terminalSmokeStatus  L74:apiDELETE  L90:removeSmokeRunTree  L106:removeSmokeManifest</sub>
- **smoke_cleanup_test.go** (81 ln)
  <sub>L13:TestRemoveSmokeRunTreeOnlyAcceptsDirectRunChild  L35:TestTerminalSmokeStatus  L48:TestCleanupSmokeOperationCancelsThenDeletes</sub>
- **web.go** (24 ln)
  <sub>L12:webUIHandler</sub>

