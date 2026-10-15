(() => {
  const boxes = [...document.querySelectorAll('.cart-select')];
  const all = document.getElementById('select-all');
  const total = document.getElementById('selected-total');
  const count = document.getElementById('selected-count');
  const button = document.getElementById('checkout-button');
  if (!all || !button) return;
  function refresh() {
    const selected = boxes.filter(b => b.checked);
    const cents = selected.reduce((sum, b) => sum + Number(b.dataset.subtotal), 0);
    total.textContent = '฿' + (cents / 100).toLocaleString('th-TH', {minimumFractionDigits: 2, maximumFractionDigits: 2});
    count.textContent = selected.length;
    button.textContent = 'สั่งซื้อ (' + selected.length + ')';
    button.disabled = selected.length === 0;
    all.checked = selected.length === boxes.length;
    all.indeterminate = selected.length > 0 && selected.length < boxes.length;
  }
  boxes.forEach(b => b.addEventListener('change', refresh));
  all.addEventListener('change', () => { boxes.forEach(b => b.checked = all.checked); refresh(); });
  refresh();
})();
