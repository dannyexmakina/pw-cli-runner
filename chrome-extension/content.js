let lastTarget = null;

document.addEventListener('contextmenu', (e) => {
  lastTarget = e.target;
}, true);

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.action !== 'copyXPath') return;
  if (!lastTarget) {
    sendResponse({ ok: false, reason: 'no target' });
    return;
  }
  const xpath = buildAbsoluteXPath(lastTarget);
  copyToClipboard(xpath);
  sendResponse({ ok: true, xpath });
});

function buildAbsoluteXPath(el) {
  const segments = [];
  let current = el;
  while (current && current.nodeType === 1 && current.tagName !== 'HTML') {
    const tag = current.tagName.toLowerCase();
    const parent = current.parentNode;
    if (!parent) break;
    const sameTag = Array.from(parent.children)
      .filter((c) => c.tagName === current.tagName);
    const index = sameTag.indexOf(current) + 1;
    segments.unshift(`${tag}[${index}]`);
    current = parent;
  }
  return '/html/' + segments.join('/');
}

function copyToClipboard(text) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed';
  ta.style.top = '-9999px';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try {
    document.execCommand('copy');
  } catch (_) {}
  document.body.removeChild(ta);
}
