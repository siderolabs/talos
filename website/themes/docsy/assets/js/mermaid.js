{{ with .Site.Params.mermaid }}
{{ if .enable }}
(function($) {
    var needMermaid = false;
    $('.language-mermaid').parent().replaceWith(function() {
        needMermaid = true;
        return $('<pre class="mermaid">').text($(this).text());
    });

    if (!needMermaid)  {
        mermaid.initialize({startOnLoad: false});
        return;
    }

    var params = {{ . | jsonify | safeJS }};

    // site params are stored with lowercase keys; lookup correct casing
    // from Mermaid default config.
    var norm = function(defaultConfig, params) {
        var result = {};
        for (const key in defaultConfig) {
            const keyLower = key.toLowerCase();
            if (defaultConfig.hasOwnProperty(key) && params.hasOwnProperty(keyLower)) {
                if (typeof defaultConfig[key] === "object") {
                    result[key] = norm(defaultConfig[key], params[keyLower]);
                } else {
                    result[key] = params[keyLower];
                }
            }
        }
        return result;
    };
    var settings = norm(mermaid.mermaidAPI.defaultConfig, params);
    settings.startOnLoad = true;
    mermaid.initialize(settings);
})(jQuery);
{{ end }}
{{ end }}
