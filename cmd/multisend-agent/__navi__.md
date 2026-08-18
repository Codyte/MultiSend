# __navi__ · `cmd/multisend-agent/` — 21 files → symbols at exact line numbers
<!-- navindex · 2026-08-18 · DO NOT EDIT BY HAND; regen via navindex skill -->
↑ repo tree: [`../../__navi__.md`](../../__navi__.md)

- **config_api.go** (332 ln)
  <sub>L33:type editableConfig  L57:type configRuntimeView  L67:type configAPIResponse  L75:app.configHandler  L117:app.loadPersistedConfig  L133:configResponse  L151:editableConfigFrom  L177:applyEditableConfig  L268:normalizeSafeDirectory  L299:normalizeStringList  L324:oneOf</sub>
- **config_api_test.go** (163 ln)
  <sub>L16:writeConfigFixture  L25:TestConfigGetRedactsNodeSecret  L47:TestConfigPutValidatesAndPreservesRuntimeFields  L105:TestConfigPutRejectsUnsafeRootWithoutChangingFile  L125:TestConfigPutRejectsRelativeReceivePath  L139:TestConfigPutGeneratesSecretWhenAuthIsEnabled</sub>
- **dashboard_api.go** (69 ln)
  <sub>L11:app.dashboardHandler  L26:app.healthSnapshot  L35:app.jobsSnapshot  L47:app.interfacesSnapshot</sub>
- **doctor.go** (639 ln)
  <sub>L54:type doctorReporter  L61:doctorReporter.line  L64:doctorReporter.Pass  L65:doctorReporter.Warn  L66:doctorReporter.Fail  L67:doctorReporter.Skip  L69:RunDoctor  L93:type runtimeState  L100:readRuntimeState  L115:printDoctorHeader  L138:printDoctorPaths  L168:printInstalledFiles  L188:validateAndPrintConfig  L259:redactDiagnosticSecrets  L283:printAgentProcess  L321:testLocalAPI  L352:printFirewallChecks  L399:firewallRuleVisibleViaNetsh  L404:printAutostart  L425:printExplorerContext  L475:printInterfaces  L493:printSummary  L509:readConfigForDoctor  L532:fileVersion …</sub>
- **doctor_test.go** (26 ln)
  <sub>L5:TestRedactDiagnosticSecrets</sub>
- **download_api.go** (241 ln)
  <sub>L14:app.receiverSessionsHandler  L22:app.receiverSessionByID  L41:app.listReceiverSessions  L54:app.getReceiverSession  L66:app.downloadsHandler  L97:app.downloadByID  L190:app.interfacesHandler  L198:app.localIPs  L210:app.enrichSendOptions</sub>
- **download_smoke.go** (348 ln)
  <sub>L27:runDownloadSmoke  L277:toFloat  L290:toInt  L304:printDownloadFailedChunks  L314:printDownloadManifestErrors</sub>
- **history.go** (581 ln)
  <sub>L55:type persistedJobState  L69:type persistedPullState  L74:type operationHistoryFile  L79:app.restoreOperationHistory  L84:app.operationHistoryDir  L91:app.persistJobLocked  L118:app.persistPullLocked  L132:saveJSONAtomic  L164:app.restoreJobHistory  L185:app.jobStateFromHistory  L256:app.validateRestoredJobOptions  L280:validP2PHistoryManifest  L321:sourceMatchesManifest  L330:manifestAllDone  L339:app.restorePullHistory  L377:recentOperationHistory  L399:validOperationID  L413:app.pruneJobsLocked  L429:app.prunePullsLocked  L445:app.forgetJob  L475:validateManagedCleanupPath  L512:removeManagedCleanupPath  L523:app.forgetPull  L548:samePath …</sub>
- **history_test.go** (312 ln)
  <sub>L27:TestSaveJSONAtomicReplacesWithoutTemporaryFiles  L49:TestOperationHistoryRestoresSafeJobAndPullContext  L117:TestOperationHistoryMarksUnsafeManifestNonResumable  L166:TestOperationHistoryRestoresCompletedJobWithoutSourceFile  L195:TestForgetOperationHistoryOnlyRemovesTerminalRecord  L227:TestForgetOperationHistoryRejectsActiveAndUnavailableStorage  L240:TestManagedLauncherCleanupPathAndForget  L282:writeP2PHistoryManifest</sub>
- **http_api.go** (160 ln)
  <sub>L16:app.localHandler  L47:app.shutdownAgent  L63:app.controlHandler  L70:newAgentHTTPServer  L82:localRequestGuard  L100:isLoopbackHost  L113:isSameRequestOrigin  L125:decodeJSONRequest  L129:decodeOptionalJSONRequest  L133:decodeJSONRequestAllowEmpty</sub>
- **http_api_test.go** (178 ln)
  <sub>L18:localRequest  L24:TestLocalHandlerRedirectsAndServesEmbeddedUI  L56:TestLocalRequestGuardRejectsExternalHostAndOrigin  L84:TestReadOnlyHandlersRejectPost  L105:TestDashboardHandlerReturnsUnifiedSnapshot  L145:TestJSONRequestBodyLimit  L154:TestShutdownAgentRespondsBeforeCancel  L170:TestNewAgentHTTPServerHasDefensiveTimeouts</sub>
- **lab_smoke.go** (531 ln)
  <sub>L36:type labRuntimeState  L41:type smokeJob  L66:type smokeResult  L72:smokeResult.passf  L80:smokeResult.warnf  L82:runLabSmoke  L362:readRuntime  L393:ensureDeterministicFile  L422:apiGET  L438:apiPOST  L455:canDial  L464:waitForSmokeReceiver  L488:printLabSummary  L501:printFailedChunkDetails</sub>
- **lab_smoke_test.go** (41 ln)
  <sub>L8:TestValidateLabReceivePath  L20:TestValidateLabReceivePathRejectOutside  L28:TestValidateLabReceivePathRejectEmpty  L35:TestValidateLabReceivePathRejectTraversal</sub>
- **main.go** (451 ln)
  <sub>L57:type channelProgress  L71:type channelsProgress  L76:type jobDetails  L103:type receiverSession  L119:type failedChunkRef  L126:type progressSample  L133:type jobState  L153:type receiverSessionState  L160:type app  L175:type pullJobState  L193:type sendTargetOptions  L202:type apiErrorResponse  L212:type preparedRemoteSource  L223:agentLogPath  L234:setupAgentLogging  L256:rotateLogIfLarge  L269:main  L334:app.selectAndPersistPorts  L364:app.writeRuntimeState  L385:app.runLocalAPI  L398:app.runControlAPI  L411:writeJSON  L416:writeAPIError  L420:writeAPIErrorDetail …</sub>
- **main_test.go** (744 ln)
  <sub>L58:TestControlJSONClientsSendBearerToken  L83:TestControlJSONClientRejectsOversizedResponse  L96:TestControlAPITokenOnlyWhenAuthRequired  L107:TestControlAuthFailsClosedAndAcceptsCaseInsensitiveScheme  L124:TestRemoteSendPathAllowedResolvesSymlinkEscape  L152:TestStartSendRejectsInvalidChunkSizeBeforeCreatingJob  L166:TestSendRejectsLabReceivePathWhenLabFalse  L193:TestSendRejectsRelativePathAndInvalidChunkSize  L223:TestSendRejectsUnsafeCleanupPath  L237:TestPullRejectsInvalidChunkSizeBeforeDiscovery  L247:TestInterfacesHandlerPayload  L292:TestWriteAPIErrorReturnsJSON  L314:TestFormatRemoteAPIErrorIncludesRequestID  L324:TestMergeReceiverChunksOrdersByIndex  L353:TestValidateReceiverFinalSize  L367:TestCleanupReceiverChunksOnlyRemovesChunks  L395:TestFinalizeReceiverSessionIdempotent  L439:TestUpdateReceiverManifestSerializesConcurrentChunks  L485:TestHandleConnPublishesVerifiedChunkAtomically  L560:TestUpdateReceiverManifestRejectsIdentityChange  L573:TestUpdateReceiverManifestPreservesCorruptManifest  L590:TestValidateReceiverPathRejectsOutside  L598:TestParseFileSourceAcceptsUNCAndFileURL  L615:TestValidateOutputUnderReceiveReturnsRelativeDestination …</sub>
- **pull.go** (812 ln)
  <sub>L56:app.pullsHandler  L88:app.pullByID  L136:app.controlAuthOK  L152:app.controlAPIToken  L159:app.remoteSendRoots  L169:app.isRemoteSendPathAllowed  L186:resolveExistingPath  L198:resolvePathWithExistingParent  L228:app.isRemoteAllowed  L250:app.resolvePeerAddressByNodeID  L268:parseFileSource  L320:prepareRemoteSendSource  L357:zipDirectory  L416:app.findPeerByHost  L453:validateOutputUnderReceive  L476:app.remoteControlBase  L489:postJSON  L509:getJSON  L524:decodeRemoteAPIResponse  L541:setBearerToken  L547:formatRemoteAPIError  L572:app.createPull  L658:validateChunkSizeMB  L665:app.pullViewFromRemote …</sub>
- **receiver.go** (762 ln)
  <sub>L56:app.runReceiver  L79:app.handleConn  L204:receiveChunkToTemp  L235:receiverChunkMatches  L253:app.publishReceiverChunk  L274:app.updateReceiverManifest  L286:app.updateReceiverManifestFile  L342:app.ensureReceiverSession  L356:app.updateReceiverSession  L369:app.maybeScheduleReceiverFinalize  L390:app.finalizeReceiverSession  L469:app.failReceiverFinalize  L478:app.markReceiverDone  L494:countManifestChunksByStatus  L504:validateReceiverManifestComplete  L520:validateReceiverFinalSize  L531:validateReceiverPath  L553:isUnder  L567:isPullHeader  L571:validateReceiveRelPath  L589:resolveReceiveRelRoot  L612:sanitizePublishName  L624:receiverTargetInfo  L643:receiverFinalPath …</sub>
- **send.go** (744 ln)
  <sub>L39:app.health  L47:app.peers  L55:app.send  L97:app.startSend  L202:app.jobs  L210:app.jobByID  L272:app.remoteSend  L345:app.remoteJobByID  L408:app.createJob  L450:app.resumeJob  L508:app.cancelJob  L529:app.updateJobProgress  L651:app.finishJob  L680:app.setJobManifest  L695:app.refreshJobFromManifestLocked</sub>
- **smoke_cleanup.go** (123 ln)
  <sub>L13:cleanupLabSmokeArtifacts  L27:cleanupDownloadSmokeArtifacts  L38:cleanupSmokeOperation  L65:terminalSmokeStatus  L74:apiDELETE  L90:removeSmokeRunTree  L106:removeSmokeManifest</sub>
- **smoke_cleanup_test.go** (81 ln)
  <sub>L13:TestRemoveSmokeRunTreeOnlyAcceptsDirectRunChild  L35:TestTerminalSmokeStatus  L48:TestCleanupSmokeOperationCancelsThenDeletes</sub>
- **web.go** (24 ln)
  <sub>L12:webUIHandler</sub>

