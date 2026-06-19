function sendToMultiSend(url) {
  // Launcher expects multisend://<url-encoded>.
  const target = `multisend://${encodeURIComponent(url)}`;
  chrome.tabs.create({ url: target });
}

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "multisend-download",
    title: "Send link to MultiSend",
    contexts: ["link"]
  });
});

chrome.contextMenus.onClicked.addListener((info) => {
  if (info.menuItemId === "multisend-download" && info.linkUrl) {
    sendToMultiSend(info.linkUrl);
  }
});

chrome.action.onClicked.addListener(async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab && tab.url) {
    sendToMultiSend(tab.url);
  }
});
