// ─── 1. Heading anchor links ──────────────────────────────────────────────
// Adds a clickable "#" link to h2, h3, h4 headings. Clicking copies the
// anchor URL to clipboard and updates the browser hash.
(function () {
  var headings = document.querySelectorAll(
    '.content-main h2[id], .content-main h3[id], .content-main h4[id]'
  );

  headings.forEach(function (h) {
    var link = document.createElement('a');
    link.className = 'heading-anchor';
    link.href = '#' + h.id;
    link.setAttribute('aria-label', 'Link to ' + h.textContent);
    link.innerHTML =
      '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>';

    link.addEventListener('click', function (e) {
      e.preventDefault();
      var url = window.location.origin + window.location.pathname + '#' + h.id;
      history.replaceState(null, '', '#' + h.id);
      h.scrollIntoView({ behavior: 'smooth', block: 'start' });

      navigator.clipboard.writeText(url).then(function () {
        showToast('Link copied!');
      });
    });

    h.style.position = 'relative';
    h.appendChild(link);
  });
})();

// ─── 2. Active TOC tracking ──────────────────────────────────────────────
// Highlights the current section in the "On this page" sidebar as you scroll.
(function () {
  var toc = document.querySelector('.toc');
  if (!toc) return;

  var tocLinks = toc.querySelectorAll('a');
  if (!tocLinks.length) return;

  var headingIds = Array.from(tocLinks).map(function (a) {
    return a.getAttribute('href').replace('#', '');
  });

  var headingElements = headingIds
    .map(function (id) {
      return document.getElementById(id);
    })
    .filter(Boolean);

  if (!headingElements.length) return;

  var observer = new IntersectionObserver(
    function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          tocLinks.forEach(function (link) {
            link.classList.remove('toc-active');
          });
          var active = toc.querySelector('a[href="#' + entry.target.id + '"]');
          if (active) active.classList.add('toc-active');
        }
      });
    },
    {
      rootMargin: '-80px 0px -70% 0px',
      threshold: 0,
    }
  );

  headingElements.forEach(function (el) {
    observer.observe(el);
  });
})();

// ─── 3. Keyboard shortcut for search (Cmd+K or /) ───────────────────────
(function () {
  var searchInput = document.getElementById('search-input');
  if (!searchInput) return;

  document.addEventListener('keydown', function (e) {
    // Don't trigger when typing in an input/textarea
    if (
      e.target.tagName === 'INPUT' ||
      e.target.tagName === 'TEXTAREA' ||
      e.target.isContentEditable
    ) {
      // Escape closes search from within the input
      if (e.key === 'Escape') {
        searchInput.blur();
        var results = document.getElementById('search-results');
        if (results) results.innerHTML = '';
        searchInput.value = '';
      }
      return;
    }

    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      searchInput.focus();
    } else if (e.key === '/') {
      e.preventDefault();
      searchInput.focus();
    } else if (e.key === 'Escape') {
      searchInput.blur();
      var results = document.getElementById('search-results');
      if (results) results.innerHTML = '';
      searchInput.value = '';
    }
  });
})();

// ─── 4. Back-to-top button ───────────────────────────────────────────────
(function () {
  var btn = document.createElement('button');
  btn.className = 'back-to-top';
  btn.setAttribute('aria-label', 'Back to top');
  btn.innerHTML =
    '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"/></svg>';

  btn.addEventListener('click', function () {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  });

  document.body.appendChild(btn);

  var scrollThreshold = 400;
  var visible = false;

  window.addEventListener(
    'scroll',
    function () {
      var shouldShow = window.scrollY > scrollThreshold;
      if (shouldShow !== visible) {
        visible = shouldShow;
        btn.classList.toggle('visible', visible);
      }
    },
    { passive: true }
  );
})();

// ─── Toast helper ────────────────────────────────────────────────────────
function showToast(message) {
  var existing = document.querySelector('.toast');
  if (existing) existing.remove();

  var toast = document.createElement('div');
  toast.className = 'toast';
  toast.textContent = message;
  document.body.appendChild(toast);

  requestAnimationFrame(function () {
    toast.classList.add('toast-visible');
  });

  setTimeout(function () {
    toast.classList.remove('toast-visible');
    setTimeout(function () {
      toast.remove();
    }, 200);
  }, 1800);
}
