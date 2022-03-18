if (typeof Storage !== 'undefined') {
    const activeLanguage = localStorage.getItem('active_language');
    if (activeLanguage) {
        document
            .querySelectorAll('.tab-' + activeLanguage)
            .forEach((element) => {
              $('#' + element.id).tab('show');
            });
    }
}
function handleClick(language) {
    if (typeof Storage !== 'undefined') {
        localStorage.setItem('active_language', language);
        document.querySelectorAll('.tab-' + language)
          .forEach((element) => {
            $('#' + element.id).tab('show');
        });
    }
}
