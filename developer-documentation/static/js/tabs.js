(function() {
  document.querySelectorAll('.code-tabs').forEach(function(tabGroup) {
    var tabs = tabGroup.querySelectorAll('.code-tab-content');
    if (tabs.length === 0) return;

    var header = tabGroup.querySelector('.code-tabs-header');
    if (!header) {
      header = document.createElement('div');
      header.className = 'code-tabs-header';
      tabGroup.insertBefore(header, tabGroup.firstChild);
    }
    header.innerHTML = '';

    tabs.forEach(function(tab, i) {
      var name = tab.getAttribute('data-tab') || 'Tab ' + (i + 1);
      var btn = document.createElement('button');
      btn.className = 'code-tab-btn' + (i === 0 ? ' active' : '');
      btn.textContent = name;
      btn.addEventListener('click', function() {
        tabGroup.querySelectorAll('.code-tab-btn').forEach(function(b) { b.classList.remove('active'); });
        tabGroup.querySelectorAll('.code-tab-content').forEach(function(t) { t.classList.remove('active'); });
        btn.classList.add('active');
        tab.classList.add('active');
      });
      header.appendChild(btn);

      if (i === 0) {
        tab.classList.add('active');
      } else {
        tab.classList.remove('active');
      }
    });
  });
})();
