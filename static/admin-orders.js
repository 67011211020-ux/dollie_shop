document.addEventListener("DOMContentLoaded", function () {
  document.querySelectorAll(".admin-order-status-form, .admin-order-payment-form").forEach(function (form) {
    const select = form.querySelector('select[name="status"]');
    const button = form.querySelector('button[type="submit"]');
    if (!select || !button) return;
    function update() { button.disabled = select.value === select.dataset.original; }
    select.addEventListener("change", update);
    update();
  });
});
