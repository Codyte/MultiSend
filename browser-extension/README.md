# MultiSend browser extension

This extension forwards links to the local `multisend://` protocol handler.

Install the payload copied by `install-multisend-node-v5.ps1` from:

- `C:\Program Files\MultiSend\browser-extension`

The protocol registration launches `multisend-launcher.ps1` with `-OpenWebUI`
and `-ProtocolUrl`. The launcher normalizes the URL and opens the local web UI
with the download form prefilled.
