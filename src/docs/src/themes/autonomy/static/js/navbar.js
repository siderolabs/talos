$(document).ready(function () {
  $("body").on("mouseenter", ".navbar-link", function (e) {
    $( this ).next().addClass('open');
    e.preventDefault()
    e.stopImmediatePropagation();
  });

  $("body").on("mouseleave", ".navbar-item", function (e) {
    $( ".navbar-popover" ).removeClass('open');
    e.preventDefault()
    e.stopImmediatePropagation();
  });
});


