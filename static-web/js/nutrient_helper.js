
function addNutrientRow() {
    const id = Date.now();
    const div = document.createElement('div');
    div.className = 'nutrient-row';
    div.style.display = 'grid';
    div.style.gridTemplateColumns = '2fr 1fr 1fr auto';
    div.style.gap = '10px';
    div.style.marginBottom = '10px';
    div.id = `nutrient-${id}`;
    div.innerHTML = `
        <input type="text" placeholder="Nutrient Name (e.g. Vitamin C)" class="nutrient-name" required>
        <input type="number" step="any" placeholder="Amount" class="nutrient-amount" required>
        <input type="text" placeholder="Unit (e.g. mg)" class="nutrient-unit" required>
        <button type="button" onclick="document.getElementById('nutrient-${id}').remove()" style="background-color: #d9534f;">X</button>
    `;
    els.nutrientsContainer.appendChild(div);
}
