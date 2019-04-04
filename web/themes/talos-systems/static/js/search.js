var options = {
    shouldSort: true,
    tokenize: true,
    findAllMatches: true,
    includeScore: true,
    includeMatches: true,
    threshold: 0.0,
    location: 0,
    distance: 100,
    maxPatternLength: 32,
    minMatchCharLength: 1,
    keys: [
        { name: "title", weight: 0.8 },
        { name: "contents", weight: 0.5 },
    ]
};

var query = param("s");
if (query) {
    $("#search-query").val(query);
    search(query);
}

function param(name) {
    return decodeURIComponent((location.search.split(name + '=')[1] || '').split('&')[0]).replace(/\+/g, ' ');
}

function search(query) {
    $.getJSON("/index.json", function (data) {
        var pages = data;
        var fuse = new Fuse(pages, options);
        var result = fuse.search(query);
        if (result.length > 0) {
            results(result, query);
        } else {
            $('#search-results').append("<p class=\"search-result-item centered\" style=\"text-align:center;\">No matches found</p>");
        }
    });
}

function make(params) {

}

function results(results, query) {
    var objs = {}

    results.forEach((element, idx) => {
        var title = element.item.title;
        var permalink = element.item.permalink
        var preview = '';

        var contents = element.item.contents;

        var highlights = [];
        highlights.push(query)

        var obj = {}
        if (title in objs) {
            obj = objs[title]
        } else {
            objs[title] = {
                'title': title,
                'link': permalink,
                'preview': '',
                'key': idx,
                'highlights': highlights,
            }
            obj = objs[title]
        }

        element.matches.forEach(match => {
            match.indices.forEach(index => {
                start = index[0]
                end = index[1] + 1

                var substring
                if (match.key == "contents") {
                    substring = contents.substring(start, end)
                    if (substring.toLowerCase().includes(query.toLowerCase())) {
                        r1 = start - 25
                        r2 = end + 25
                        rangeStart = r1 > 0 ? r1 : 0;
                        rangeEnd = r2 < contents.length ? r2 : contents.length;
                        obj['preview'] += "..." + contents.substring(rangeStart, rangeEnd) + "..." + '\n'
                    }
                }
            });
        });
    });

    // Render the HTML.

    for (const title in objs) {
        if (objs.hasOwnProperty(title)) {
            const obj = objs[title];
            var tpl = $('#search-result-template').html();
            var output = render(tpl, { key: obj['key'], title: obj['title'], link: obj['link'], preview: obj['preview'] });
            $('#search-results').append(output);
            $.each(obj['highlights'], function (_, value) {
                $("#summary-" + obj['key']).mark(value);
            });
        }
    }
}

function render(tpl, data) {
    var matches, pattern, copy;
    pattern = /\$\{\s*isset ([a-zA-Z]*) \s*\}(.*)\$\{\s*end\s*}/g;
    //since loop below depends on re.lastInxdex, we use a copy to capture any manipulations whilst inside the loop
    copy = tpl;
    while ((matches = pattern.exec(tpl)) !== null) {
        if (data[matches[1]]) {
            //valid key, remove conditionals, leave contents.
            copy = copy.replace(matches[0], matches[2]);
        } else {
            //not valid, remove entire section
            copy = copy.replace(matches[0], '');
        }
    }
    tpl = copy;
    //now any conditionals removed we can do simple substitution
    var key, find, re;
    for (key in data) {
        find = '\\$\\{\\s*' + key + '\\s*\\}';
        re = new RegExp(find, 'g');
        tpl = tpl.replace(re, data[key]);
    }

    return tpl;
}
