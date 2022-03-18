{{with .Site.Params.drawio}}
{{if .enable }}
(function () {
  var shade;
  var iframe;

  var insertFrame = function () {
    shade = document.createElement('div');
    shade.classList.add('drawioframe');
    iframe = document.createElement('iframe');
    shade.appendChild(iframe);
    document.body.appendChild(shade);
  }

  var closeFrame = function () {
    if (shade) {
      document.body.removeChild(shade);
      shade = undefined;
      iframe = undefined;
    }
  }

  var imghandler = function (img, imgdata) {
    var url = {{ .drawio_server | default "https://embed.diagrams.net/" | jsonify }};
    url += '?embed=1&ui=atlas&spin=1&modified=unsavedChanges&proto=json&saveAndEdit=1&noSaveBtn=1';

    var wrapper = document.createElement('div');
    wrapper.classList.add('drawio');
    img.parentNode.insertBefore(wrapper, img);
    wrapper.appendChild(img);

    var btn = document.createElement('button');
    btn.classList.add('drawiobtn');
    btn.insertAdjacentHTML('beforeend', '<i class="fas fa-edit"></i>');
    wrapper.appendChild(btn);

    btn.addEventListener('click', function (evt) {
      if (iframe) return;
      insertFrame();
      var handler = function (evt) {
        var wind = iframe.contentWindow;
        if (evt.data.length > 0 && evt.source == wind) {
          var msg = JSON.parse(evt.data);

          if (msg.event == 'init') {
            wind.postMessage(JSON.stringify({action: 'load', xml: imgdata}), '*');

          } else if (msg.event == 'save') {
            var fmt = imgdata.indexOf('data:image/png') == 0 ? 'xmlpng' : 'xmlsvg';
            wind.postMessage(JSON.stringify({action: 'export', format: fmt}), '*');

          } else if (msg.event == 'export') {
            const fn = img.src.replace(/^.*?([^/]+)$/, '$1');
            const dl = document.createElement('a');
            dl.setAttribute('href', msg.data);
            dl.setAttribute('download', fn);
            document.body.appendChild(dl);
            dl.click();
            dl.parentNode.removeChild(dl);
          }

          if (msg.event == 'exit' || msg.event == 'export') {
            window.removeEventListener('message', handler);
            closeFrame();
          }
        }
      };

      window.addEventListener('message', handler);
      iframe.setAttribute('src', url);
    });
  };


document.addEventListener('DOMContentLoaded', function () {
  // find all the png and svg images that may have embedded xml diagrams
  for (const el of document.getElementsByTagName('img')) {
    const img = el;
    const src = img.getAttribute('src');
    if (!src.endsWith('.svg') && !src.endsWith('.png')) {
      continue;
    }

    const xhr = new XMLHttpRequest();
    xhr.responseType = 'blob';
    xhr.open("GET", src);
    xhr.addEventListener("load", function () {
      const fr = new FileReader();
      fr.addEventListener('load', function () {
        if (fr.result.indexOf('mxfile') != -1) {
          const fr = new FileReader();
          fr.addEventListener('load', function () {
            const imgdata = fr.result;
            imghandler(img, imgdata);
          });
          fr.readAsDataURL(xhr.response);
        }
      });
      fr.readAsBinaryString(xhr.response);
    });
    xhr.send();
  };
});
}());
{{end}}
{{end}}
