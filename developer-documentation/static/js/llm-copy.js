// LLM-friendly markdown copy — copies the raw markdown source of the current page
// to the clipboard so users can paste it into their LLM of choice.
(function () {
  document.querySelectorAll('.llm-copy-btn').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var source = btn.closest('article, .content-main')
        ? btn.closest('article, .content-main').querySelector('.llm-markdown-source')
        : btn.parentElement.querySelector('.llm-markdown-source');
      if (!source) return;

      var markdown = source.value || source.textContent;
      navigator.clipboard.writeText(markdown).then(function () {
        btn.classList.add('copied');
        setTimeout(function () {
          btn.classList.remove('copied');
        }, 2000);
      });
    });
  });
})();
