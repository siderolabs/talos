---
title: "Docsy Shortcodes"
linkTitle: "Docsy Shortcodes"
date: 2017-01-05
weight: 5
description: >
  Use Docsy's Hugo shortcodes to quickly build site pages.
resources:
- src: "**spruce*.jpg"
  params:
    byline: "Photo: Bjørn Erik Pedersen / CC-BY-SA"
---

Rather than writing all your site pages from scratch, Hugo lets you define and use [shortcodes](https://gohugo.io/content-management/shortcodes/). These are reusable snippets of content that you can include in your pages, often using HTML to create effects that are difficult or impossible to do in simple Markdown. Shortcodes can also have parameters that let you, for example, add your own text to a fancy shortcode text box. As well as Hugo's [built-in shortcodes](https://gohugo.io/content-management/shortcodes/), Docsy provides some shortcodes of its own to help you build your pages.

## Shortcode blocks

The theme comes with a set of custom  **Page Block** shortcodes that can be used to compose landing pages, about pages, and similar.

These blocks share some common parameters:

height
: A pre-defined height of the block container. One of `min`, `med`, `max`, `full`, or `auto`. Setting it to `full` will fill the Viewport Height, which can be useful for landing pages.

color
: The block will be assigned a color from the theme palette if not provided, but you can set your own if needed. You can use all of Bootstrap's color names, theme color names or a grayscale shade. Some examples would be `primary`, `white`, `dark`, `warning`, `light`, `success`, `300`, `blue`, `orange`. This will become the **background color** of the block, but text colors will adapt to get proper contrast.

### blocks/cover

The **blocks/cover** shortcode creates a landing page type of block that fills the top of the page.

```html
{{</* blocks/cover title="Welcome!" image_anchor="center" height="full" color="primary" */>}}
<div class="mx-auto">
	<a class="btn btn-lg btn-primary mr-3 mb-4" href="{{</* relref "/docs" */>}}">
		Learn More <i class="fas fa-arrow-alt-circle-right ml-2"></i>
	</a>
	<a class="btn btn-lg btn-secondary mr-3 mb-4" href="https://example.org">
		Download <i class="fab fa-github ml-2 "></i>
	</a>
	<p class="lead mt-5">This program is now available in <a href="#">AppStore!</a></p>
	<div class="mx-auto mt-5">
		{{</* blocks/link-down color="info" */>}}
	</div>
</div>
{{</* /blocks/cover */>}}
```

Note that the relevant shortcode parameters above will have sensible defaults, but is included here for completeness.

{{% alert title="Hugo Tip" %}}
> Using the bracket styled shortcode delimiter, `>}}`, tells Hugo that the inner content is HTML/plain text and needs no further processing. Changing the delimiter to `%}}` means Hugo will treat the content as Markdown. You can use both styles in your pages.
{{% /alert %}}


| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| title | | The main display title for the block. | 
| image_anchor | |
| height | | See above.
| color | | See above. 
| byline | Byline text on featured image. |


To set the background image, place an image with the word "background" in the name in the page's [Page Bundle](/docs/adding-content/content/#page-bundles). For example, in our the example site the background image in the home page's cover block is [`featured-background.jpg`](https://github.com/google/docsy-example/tree/master/content/en), in the same directory.

{{% alert title="Tip" %}}
If you also include the word **featured** in the image name, e.g. `my-featured-background.jpg`, it will also be used as the Twitter Card image when shared.
{{% /alert %}}

For available icons, see [Font Awesome](https://fontawesome.com/icons?d=gallery&m=free).

### blocks/lead

The **blocks/lead** block shortcode is a simple lead/title block with centred text and an arrow down pointing to the next section.

```go-html-template
{{%/* blocks/lead color="dark" */%}}
TechOS is the OS of the future. 

Runs on **bare metal** in the **cloud**!
{{%/* /blocks/lead */%}}
```

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| height | | See above.
| color | | See above. 

### blocks/section

The **blocks/section** shortcode is meant as a general-purpose content container. It comes in two "flavors", one for general content and one with styling more suitable for wrapping a horizontal row of feature sections.

The example below shows a section wrapping 3 feature sections.


```go-html-template
{{</* blocks/section color="dark" */>}}
{{%/* blocks/feature icon="fa-lightbulb" title="Fastest OS **on the planet**!" */%}}
The new **TechOS** operating system is an open source project. It is a new project, but with grand ambitions.
Please follow this space for updates!
{{%/* /blocks/feature */%}}
{{%/* blocks/feature icon="fab fa-github" title="Contributions welcome!" url="https://github.com/gohugoio/hugo" */%}}
We do a [Pull Request](https://github.com/gohugoio/hugo/pulls) contributions workflow on **GitHub**. New users are always welcome!
{{%/* /blocks/feature */%}}
{{%/* blocks/feature icon="fab fa-twitter" title="Follow us on Twitter!" url="https://twitter.com/GoHugoIO" */%}}
For announcement of latest features etc.
{{%/* /blocks/feature */%}}
{{</* /blocks/section */>}}
```

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| height | | See above.
| color | | See above. 
| type  | | Specify "section" if you want a general container,  omit this parameter if you want this section to contain a horizontal row of features.

### blocks/feature

```go-html-template

{{%/* blocks/feature icon="fab fa-github" title="Contributions welcome!" url="https://github.com/gohugoio/hugo" */%}}
We do a [Pull Request](https://github.com/gohugoio/hugo/pulls) contributions workflow on **GitHub**. New users are always welcome!
{{%/* /blocks/feature */%}}

```

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| title | | The title to use.
| url | | The URL to link to.
| icon | | The icon class to use.


### blocks/link-down

The **blocks/link-down** shortcode creates a navigation link down to the next section. It's meant to be used in combination with the other blocks shortcodes.

```go-html-template

<div class="mx-auto mt-5">
	{{</* blocks/link-down color="info" */>}}
</div>
```

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| color | info | See above. 

## Shortcode helpers

###  alert

The **alert** shortcode creates an alert block that can be used to display notices or warnings.

```go-html-template
{{%/* alert title="Warning" color="warning" */%}}
This is a warning.
{{%/* /alert */%}}

```

Renders to:

{{% alert title="Warning" color="warning" %}}
This is a warning.
{{% /alert %}}

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| color | primary | One of the theme colors, eg `primary`, `info`, `warning` etc.

###  pageinfo

The **pageinfo** shortcode creates a text box that you can use to add banner information for a page: for example, letting users know that the page contains placeholder content, that the content is deprecated, or that it documents a beta feature.

```go-html-template
{{%/* pageinfo color="primary" */%}}
This is placeholder content.
{{%/* /pageinfo */%}}

```

Renders to:

{{% pageinfo color="primary" %}}
This is placeholder content
{{% /pageinfo %}}

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| color | primary | One of the theme colors, eg `primary`, `info`, `warning` etc.


###  imgproc

The **imgproc** shortcode finds an image in the current [Page Bundle](/docs/adding-content/content/#page-bundles) and scales it given a set of processing instructions.


```go-html-template
{{</* imgproc spruce Fill "400x450" */>}}
Norway Spruce Picea abies shoot with foliage buds.
{{</* /imgproc */>}}
```

{{< imgproc spruce Fill "400x450" >}}
Norway Spruce Picea abies shoot with foliage buds.
{{< /imgproc >}}

The example above has also a byline with photo attribution added. When using illustrations with a free license from [WikiMedia](https://commons.wikimedia.org/) and similar, you will in most situations need a way to attribute the author or licensor. You can add metadata to your page resources in the page front matter. The `byline` param is used by convention in this theme:


```yaml
resources:
- src: "**spruce*.jpg"
  params:
    byline: "Photo: Bjørn Erik Pedersen / CC-BY-SA"
```


| Parameter        | Description  |
| ----------------: |------------|
| 1 | The image filename or enough of it to identify it (we do Glob matching)
| 2 | Command. One of `Fit`, `Resize` or `Fill`. See [Image Processing Methods](https://gohugo.io/content-management/image-processing/#image-processing-methods).
| 3 | Processing options, e.g. `400x450`. See [Image Processing Options](https://gohugo.io/content-management/image-processing/#image-processing-methods).

### swaggerui

The `swaggerui` shortcode can be placed anywhere inside a page with the [`swagger` layout](https://github.com/google/docsy/tree/master/layouts/swagger); it renders [Swagger UI](https://swagger.io/tools/swagger-ui/) using any OpenAPI YAML or JSON file as source. This can be hosted anywhere you like, for example in your site's root [`/static` folder](/docs/adding-content/content/#adding-static-content).

```yaml
---
title: "Pet Store API"
type: swagger
weight: 1
description: Reference for the Pet Store API
---

{{</* swaggerui src="/openapi/petstore.yaml" */>}}
```

You can customize Swagger UI's look and feel by overriding Swagger's CSS or by editing and compiling a [Swagger UI dist](https://github.com/swagger-api/swagger-ui) yourself and replace `themes/docsy/static/css/swagger-ui.css`.

### iframe

With this shortcode you can embed external content into a Docsy page as an inline frame (`iframe`) - see: https://www.w3schools.com/tags/tag_iframe.asp

| Parameter        | Default    | Description  |
| ---------------- |------------| ------------|
| src | | URL of external content
| width | 100% | Width of iframe
| tryautoheight | true | If true the shortcode tries to calculate the needed height for the embedded content using JavaScript, as described here: https://stackoverflow.com/a/14618068. This is only possible if the embedded content is [on the same domain](https://stackoverflow.com/questions/22086722/resize-cross-domain-iframe-height). Note that even if the embedded content is on the same domain, it depends on the structure of the content if the height can be calculated correctly.
| style | min-height:98vh; border:none; | CSS styles for the iframe. `min-height:98vh;` is a backup if `tryautoheight` doesn't work. `border:none;` removes the border from the iframe - this is useful if you want the embedded content to look more like internal content from your page.
| sandbox | false | You can switch the sandbox completely on by setting `sandbox = true` or allow specific functionality with the common values for the iframe parameter `sandbox` defined in the [HTML standard](https://www.w3schools.com/tags/att_iframe_sandbox.asp).
| name | iframe-name | Specify the [name of the iframe](https://www.w3schools.com/tags/att_iframe_name.asp).
| id | iframe-id | Sets the ID of the iframe.
| class |  | Optional parameter to set the classes of the iframe.
| sub | Your browser cannot display embedded frames. You can access the embedded page via the following link: | The text displayed (in addition to the embedded URL) if the user's browser can't display embedded frames.

{{% alert title="Warning" color="warning" %}}
You can only embed external content from a server when its `X-Frame-Options` is not set or if it specifically allows embedding for your site. See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Frame-Options for details.

There are several tools you can use to check if a website can be embedded via iframe - e.g.: https://gf.dev/x-frame-options-test. Be aware that when this test says "Couldn’t find the X-Frame-Options header 
in the response headers." you __CAN__ embed it, but when the test says "Great! X-Frame-Options header was found in the HTTP response headers as highlighted below.", you __CANNOT__ - unless it has been explicitly enabled for your site.
{{% /alert %}}

## Tabbed panes

Sometimes it's very useful to have tabbed panes when authoring content. One common use-case is to show multiple syntax highlighted code blocks that showcase the same problem, and how to solve it in different programming languages. As an example, the table below shows the language-specific variants of the famous `Hello world!` program one usually writes first when learning a new programming language from scratch:

{{< tabpane langEqualsHeader=true >}}
  {{< tab header="C" >}}
#include <stdio.h>
#include <stdlib.h>

int main(void)
{
  puts("Hello World!");
  return EXIT_SUCCESS;
}
{{< /tab >}}
{{< tab header="C++" >}}
#include <iostream>

int main()
{
  std::cout << "Hello World!" << std::endl;
}
{{< /tab >}}
{{< tab header="Go" >}}
package main
import "fmt"
func main() {
  fmt.Printf("Hello World!\n")
}
{{< /tab >}}
{{< tab header="Java" >}}
class HelloWorld {
  static public void main( String args[] ) {
    System.out.println( "Hello World!" );
  }
}
{{< /tab >}}
{{< tab header="Kotlin" >}}
fun main(args : Array<String>) {
    println("Hello, world!")
}
{{< /tab >}}
{{< tab header="Lua" >}}
print "Hello world"
{{< /tab >}}
{{< tab header="PHP" >}}
<?php
echo 'Hello World!';
?>
{{< /tab >}}
{{< tab header="Python" >}}
print("Hello World!")
{{< /tab >}}
{{< tab header="Ruby" >}}
puts "Hello World!"
{{< /tab >}}
{{< tab header="Scala" >}}
object HelloWorld extends App {
  println("Hello world!")
}
{{< /tab >}}
{{< /tabpane >}}

The Docsy template provides two shortcodes `tabpane` and `tab` that let you easily create tabbed panes. To see how to use them, have a look at the following code block, which renders to a pane with three tabs:

```go-html-template
{{</* tabpane */>}}
  {{</* tab header="English" */>}}
    Welcome!
  {{</* /tab */>}}
  {{</* tab header="German" */>}}
    Herzlich willkommen!
  {{</* /tab */>}}
  {{</* tab header="Swahili" */>}}
    Karibu sana!
  {{</* /tab */>}}
{{</* /tabpane */>}}
```

This code translates to the tabbed pane below, showing a `Welcome!` greeting in English, German or Swahili:

{{< tabpane >}}
{{< tab header="English" >}}
Welcome!
{{< /tab >}}
{{< tab  header="German" lang="de" >}}
Herzlich willkommen!
{{< /tab >}}
{{< tab  header="Swahili" >}}
Karibu sana!
{{< /tab >}}
{{< /tabpane >}}

### Shortcode details

Tabbed panes are implemented using two shortcodes:

* The `tabpane` shortcode, which is the container element for the tabs. This shortcode can optionally held the named parameters `lang` and/or `highlight`. The values of these optional parameters are passed on as second `LANG` and third `OPTIONS` arguments to Hugo's built-in [`highlight`](https://gohugo.io/functions/highlight/) function which is used to render the code blocks of the individual tabs. In case the header text of the tab equals the `language` used in the tab's code block (as in the first tabbed pane example above), you may specify `langEqualsHeader=true` in the surrounding `tabpane` shortcode. Then, the header text of the individual tab is automatically set as `language` parameter of the respective tab.
* The various `tab` shortcodes which actually represent the tabs you would like to show. We recommend specifying the named parameter `header` for each text in order to set the header text of each tab. If needed, you can additionally specify the named parameters `lang` and `highlight` for each tab. This allows you to overwrite the settings given in the parent `tabpane` shortcode. If the language is neither specified in the `tabpane` nor in the `tab`shortcode, it defaults to Hugo's site variable `.Site.Language.Lang`.

## Card panes

When authoring content, it's sometimes very useful to put similar text blocks or code fragments on card like elements, which can be optionally presented side by side. Let's showcase this feature with the following sample card group which shows the first four Presidents of the United States:

{{< cardpane >}}
{{< card header="**George Washington**" title="\*1732 &nbsp;&nbsp;&nbsp; †1799" subtitle="**President:** 1789 – 1797" footer="![SignatureGeorgeWashington](https://upload.wikimedia.org/wikipedia/commons/thumb/2/2e/George_Washington_signature.svg/320px-George_Washington_signature.svg.png \"Signature George Washington\")">}}
![PortraitGeorgeWashington](https://upload.wikimedia.org/wikipedia/commons/thumb/b/b6/Gilbert_Stuart_Williamstown_Portrait_of_George_Washington.jpg/633px-Gilbert_Stuart_Williamstown_Portrait_of_George_Washington.jpg "Portrait George Washington")
{{< /card >}}
{{< card header="**John Adams**" title="\* 1735 &nbsp;&nbsp;&nbsp; † 1826" subtitle="**President:** 1797 – 1801" footer="![SignatureJohnAdams](https://upload.wikimedia.org/wikipedia/commons/thumb/e/e8/John_Adams_Sig_2.svg/320px-John_Adams_Sig_2.svg.png \"Signature John Adams\")" >}}
![PortraitJohnAdams](https://upload.wikimedia.org/wikipedia/commons/thumb/f/ff/Gilbert_Stuart%2C_John_Adams%2C_c._1800-1815%2C_NGA_42933.jpg/633px-Gilbert_Stuart%2C_John_Adams%2C_c._1800-1815%2C_NGA_42933.jpg "Portrait John Adams")
{{< /card >}}
{{< card header="**Thomas Jefferson**" title="\* 1743 &nbsp;&nbsp;&nbsp; † 1826" subtitle="**President:** 1801 – 1809" footer="![SignatureThomasJefferson](https://upload.wikimedia.org/wikipedia/commons/thumb/0/0d/Thomas_Jefferson_Signature.svg/320px-Thomas_Jefferson_Signature.svg.png \"Signature Thomas Jefferson\")" >}}
![PortraitThomasJefferson](https://upload.wikimedia.org/wikipedia/commons/thumb/b/b1/Official_Presidential_portrait_of_Thomas_Jefferson_%28by_Rembrandt_Peale%2C_1800%29%28cropped%29.jpg/390px-Official_Presidential_portrait_of_Thomas_Jefferson_%28by_Rembrandt_Peale%2C_1800%29%28cropped%29.jpg "Portrait Thomas Jefferson")
{{< /card >}}
{{< card header="**James Madison**" title="\* 1751 &nbsp;&nbsp;&nbsp; † 1836" subtitle="**President:** 1809 – 1817" footer="![SignatureJamesMadison](https://upload.wikimedia.org/wikipedia/commons/thumb/3/39/James_Madison_sig.svg/320px-James_Madison_sig.svg.png \"Signature James Madison\")" >}}
![PortraitJamesMadison](https://upload.wikimedia.org/wikipedia/commons/thumb/2/20/James_Madison%28cropped%29%28c%29.jpg/393px-James_Madison%28cropped%29%28c%29.jpg "Portrait James Madison")
{{< /card >}}
{{< /cardpane >}}

Docsy supports creating such card panes via different shortcodes:

* The `cardpane` shortcode which is the container element for the various cards to be presented.
* The `card` shortcodes, with each shortcode representing an individual card. While cards are often presented inside a card group, a single card may stand on its own, too. A `card` shortcode can held text, images or any other arbitrary kind of markdown or HTML markup as content. If your content is programming code, you are advised to make use of the `card-code` shortcode, a special kind of card with code-highlighting and other optional features like line numbers, highlighting of certain lines, ….

### Shortcode `card` (for text, images, …)

As stated above, a card is coded using one of the shortcode `card` or `card-code`.
If your content is any kind of text other than programming code, use the universal `card`shortcode. The following code sample demonstrates how to code a card element:

```go-html-template
{{</* card header="**Imagine**" title="Artist and songwriter: John Lennon" subtitle="Co-writer: Yoko Ono"
          footer="![SignatureJohnLennon](https://server.tld/…/signature.png \"Signature John Lennon\")">*/>}}
Imagine there's no heaven, It's easy if you try<br/>
No hell below us, above us only sky<br/>
Imagine all the people living for today…

…
{{</* /card */>}}
```
This code translates to the left card shown below, showing the lyrics of John Lennon's famous song `Imagine`. A second explanatory card element to the right indicates and explains the individual components of a card:

{{< cardpane >}}
{{< card header="**Imagine**" title="Artist and songwriter: John Lennon" subtitle="Co-writer: Yoko Ono" footer="![SignatureJohnLennon](https://upload.wikimedia.org/wikipedia/commons/thumb/5/51/Firma_de_John_Lennon.svg/320px-Firma_de_John_Lennon.svg.png \"Signature John Lennon\")">}}
Imagine there's no heaven, It's easy if you try<br/>
No hell below us, above us only sky<br/>
Imagine all the people living for today…

Imagine there's no countries, it isn't hard to do<br/>
Nothing to kill or die for, and no religion too<br/>
Imagine all the people living life in peace…

Imagine no possessions, I wonder if you can<br/>
No need for greed or hunger - a brotherhood of man<br/>
Imagine all the people sharing all the world… 

You may say I'm a dreamer, but I'm not the only one<br/>
I hope someday you'll join us and the world will live as one
{{< /card >}}
{{< card header="**Header**: specified via named parameter `Header`" title="**Card title**: specified via named parameter `title`" subtitle="**Card subtitle**: specified via named parameter `subtitle`" footer="**Footer**: specified via named parameter `footer`" >}}
  **Content**: inner content of the shortcode, this may be formatted text, images, videos, … . If the extension of your page file equals `.md`, markdown format is expected, otherwise, your content will be treated as plain HTML.
{{< /card >}}
{{< /cardpane >}}

While the main content of the card is taken from the inner markup of the `card` shortcode, the optional elements `footer`, `header`, `title`, and `subtitle` are all specified as named parameters of the shortcode.

### Shortcode `card-code` (for programming code)

In case you want to display programming code on your card, a special shortcode `card-code` is provided for this purpose. The following sample demonstrates how to code a card element with the famous `Hello world!`application coded in C:

```go-html-template
{{</* card-code header="**C**" lang="C" */>}}
#include <stdio.h>
#include <stdlib.h>

int main(void)
{
  puts("Hello World!");
  return EXIT_SUCCESS;
}
{{</* /card-code */>}}
```

This code translates to the card shown below:

{{< card-code header="**C**" lang="C" highlight="" >}}
#include <stdio.h>
#include <stdlib.h>

int main(void)
{
  puts("Hello World!");
  return EXIT_SUCCESS;
}
{{< /card-code >}}

<br/>The `card-code` shortcode can optionally held the named parameters `lang` and/or `highlight`. The values of these optional parameters are passed on as second `LANG` and third `OPTIONS` arguments to Hugo's built-in [`highlight`](https://gohugo.io/functions/highlight/) function which is used to render the code block presented on the card.

### Card groups

Displaying two ore more cards side by side can be easily achieved by putting them between the opening and closing elements of a `cardpane` shortcode.
The general markup of a card group resembles closely the markup of a tabbed pane:

```go-html-template
{{</* cardpane */>}}
  {{</* card header="Header card 1" */>}}
    Content card 1
  {{</* /card */>}}
  {{</* card header="Header card 2" */>}}
    Content card 2
  {{</* /card */>}}
  {{</* card header="Header card 3" */>}}
    Content card 3
  {{</* /card */>}}
{{</* /cardpane */>}}
```

Contrary to tabs, cards are presented side by side, however. This is especially useful it you want to compare different programming techniques (traditional vs. modern) on two cards, like demonstrated in the example above:

{{< cardpane >}}
{{< card-code header="**Java 5**" >}}
File[] hiddenFiles = new File("directory_name")
  .listFiles(new FileFilter() {
    public boolean accept(File file) {
      return file.isHidden();
    }
  });
{{< /card-code >}}
{{< card-code header="**Java 8, Lambda expression**" >}}
File[] hiddenFiles = new File("directory_name")
  .listFiles(File::isHidden);
{{< /card-code >}}
{{< /cardpane >}}

