(function() {
  var input = document.getElementById('search-input');
  var results = document.getElementById('search-results');
  if (!input || !results) return;

  var searchIndex = null;

  function loadIndex() {
    if (searchIndex) return Promise.resolve(searchIndex);
    return fetch('/index.json')
      .then(function(r) { return r.json(); })
      .then(function(data) { searchIndex = data; return data; })
      .catch(function() { searchIndex = []; return []; });
  }

  function search(query) {
    if (!query || !searchIndex) return [];
    var q = query.toLowerCase();
    return searchIndex
      .filter(function(item) {
        return item.title.toLowerCase().indexOf(q) !== -1 ||
               item.content.toLowerCase().indexOf(q) !== -1;
      })
      .slice(0, 8);
  }

  var debounceTimer;
  input.addEventListener('input', function() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(function() {
      loadIndex().then(function() {
        var hits = search(input.value.trim());
        if (hits.length === 0 || input.value.trim() === '') {
          results.classList.remove('open');
          results.innerHTML = '';
          return;
        }
        results.innerHTML = hits.map(function(h) {
          return '<a class="search-result-item" href="' + h.url + '">' +
                 '<div class="search-result-title">' + escapeHtml(h.title) + '</div>' +
                 '<div class="search-result-section">' + escapeHtml(h.section) + '</div>' +
                 '</a>';
        }).join('');
        results.classList.add('open');
      });
    }, 200);
  });

  input.addEventListener('focus', function() {
    loadIndex();
  });

  document.addEventListener('click', function(e) {
    if (!e.target.closest('.header-search')) {
      results.classList.remove('open');
    }
  });

  function escapeHtml(str) {
    var div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }
})();
