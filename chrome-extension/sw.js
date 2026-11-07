chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: 'copyXPath',
    title: 'XPathをコピー',
    contexts: ['all']
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId !== 'copyXPath' || !tab?.id) return;
  chrome.tabs.sendMessage(
    tab.id,
    { action: 'copyXPath' },
    { frameId: info.frameId }
  );
});
