import { api } from './api.js';
import { showToast, showConfirm, DOMBuffer } from './ui.js';
import { FoodSearch } from './food-search.js';

let recipeIngredients = []; // Array of {id, name, amount, unit, calories, protein, carbs, fat, measurement_amount}
let editingFoodId = null; // null = create mode, string = edit mode
let ingredientSearch = null;
let selectedIngredientFood = null;
let ingredientPortions = [];
let ingredientSelectSeq = 0;

async function loadFoods() {
    try {
        const foods = await api.getFoods({ mine: true });
        const foodsList = document.getElementById('foods-list');
        const buffer = new DOMBuffer(foodsList);

        if (foods && foods.length > 0) {
            foods.forEach(food => {
                const li = document.createElement('li');

                const foodInfo = document.createElement('div');
                foodInfo.className = 'food-info';

                const nameEl = document.createElement('strong');
                nameEl.textContent = food.name;
                foodInfo.appendChild(nameEl);
                foodInfo.appendChild(document.createElement('br'));

                const detailsEl = document.createElement('small');
                const servingAmt = food.measurement_amount || 1;
                const servingUnit = food.measurement_unit || 'serving';
                const servingDesc = servingAmt === 1 ? servingUnit : `${servingAmt} ${servingUnit}`;
                detailsEl.appendChild(document.createTextNode(
                    `${Math.round(food.calories)} kcal | P: ${Math.round(food.protein)}g | C: ${Math.round(food.carbs)}g | F: ${Math.round(food.fat)}g · per ${servingDesc}`
                ));
                if (food.type === 'recipe') {
                    const badge = document.createElement('span');
                    badge.style.cssText = 'background:#eee; padding:2px 6px; border-radius:4px; font-size:0.8em; margin-left:4px;';
                    const servingsLabel = food.servings > 1 ? ` (makes ${food.servings})` : '';
                    badge.textContent = `Recipe${servingsLabel}`;
                    detailsEl.appendChild(badge);
                }
                foodInfo.appendChild(detailsEl);

                if (food.nutrients && food.nutrients.length > 0) {
                    foodInfo.appendChild(document.createElement('br'));
                    const nutrientsEl = document.createElement('small');
                    const em = document.createElement('em');
                    em.textContent = food.nutrients.map(n => `${n.name}: ${n.amount}${n.unit}`).join(', ');
                    nutrientsEl.appendChild(em);
                    foodInfo.appendChild(nutrientsEl);
                }

                const actionsDiv = document.createElement('div');
                actionsDiv.className = 'food-actions';

                const editBtn = document.createElement('button');
                editBtn.textContent = 'Edit';
                editBtn.className = 'btn btn-secondary btn-sm';
                editBtn.onclick = () => startEdit(food.id);
                actionsDiv.appendChild(editBtn);

                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Delete';
                deleteBtn.className = 'btn btn-danger btn-sm';
                deleteBtn.onclick = () => deleteFood(food.id);
                actionsDiv.appendChild(deleteBtn);

                li.appendChild(foodInfo);
                li.appendChild(actionsDiv);
                buffer.append(li);
            });
        } else {
            const li = document.createElement('li');
            li.textContent = 'No foods found.';
            buffer.append(li);
        }
        buffer.clearAndFlush();
    } catch (e) {
        console.error("Failed to load foods:", e);
        showToast("Failed to load foods", "error");
    }
}

function addNutrientRow(name, amount, unit) {
    const container = document.getElementById('nutrients-container');
    const row = document.createElement('div');
    row.className = 'nutrient-row';

    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.placeholder = 'Name (e.g. Vitamin C)';
    nameInput.className = 'nutrient-name';
    nameInput.required = true;
    nameInput.value = name || '';

    const amountInput = document.createElement('input');
    amountInput.type = 'number';
    amountInput.placeholder = 'Amount';
    amountInput.className = 'nutrient-amount';
    amountInput.step = '0.1';
    amountInput.required = true;
    amountInput.value = amount || '';

    const unitInput = document.createElement('input');
    unitInput.type = 'text';
    unitInput.placeholder = 'Unit (e.g. mg)';
    unitInput.className = 'nutrient-unit';
    unitInput.required = true;
    unitInput.value = unit || '';

    const removeBtn = document.createElement('button');
    removeBtn.type = 'button';
    removeBtn.className = 'btn btn-danger btn-sm';
    removeBtn.textContent = '×';
    removeBtn.onclick = () => row.remove();

    row.appendChild(nameInput);
    row.appendChild(amountInput);
    row.appendChild(unitInput);
    row.appendChild(removeBtn);
    container.appendChild(row);
}

async function deleteFood(id) {
    const confirmed = await showConfirm("Are you sure you want to delete this food?");
    if (!confirmed) return;
    try {
        await api.deleteFood(id);
        loadFoods();
    } catch (e) {
        showToast("Failed to delete food: " + e.message, 'error');
    }
}

// --- Edit Logic ---

async function startEdit(foodId) {
    // Fetch full food details (includes nutrients and ingredients)
    const food = await api.getFood(foodId);
    if (!food) { showToast("Could not load food details.", 'error'); return; }

    editingFoodId = food.id;
    const form = document.getElementById('create-food-form');

    // Update heading and button
    document.getElementById('form-heading').textContent = 'Edit Food';
    document.getElementById('form-submit-btn').textContent = 'Save Changes';
    document.getElementById('cancel-edit-btn').style.display = 'inline-flex';

    // Set name
    form.name.value = food.name;

    // Set type toggle
    if (food.type === 'recipe') {
        document.getElementById('type-recipe').checked = true;
    } else {
        document.getElementById('type-food').checked = true;
    }
    toggleFoodType();

    if (food.type === 'recipe') {
        resetIngredientPicker();

        // Populate servings
        document.getElementById('recipe-servings').value = food.servings || 1;

        // Populate serving size
        const recipeSection = document.getElementById('ingredients-section');
        recipeSection.querySelector('[name="measurement_amount"]').value = food.measurement_amount || 1;
        recipeSection.querySelector('[name="measurement_unit"]').value = food.measurement_unit || 'serving';

        // Populate ingredients
        recipeIngredients = [];
        if (food.ingredients && food.ingredients.length > 0) {
            for (const ing of food.ingredients) {
                let ingredientFood = null;
                // Fetch the ingredient food individually
                try {
                    ingredientFood = await api.getFood(ing.ingredient_id);
                } catch (err) { /* ignore */ }
                recipeIngredients.push({
                    id: ing.ingredient_id,
                    name: ingredientFood ? ingredientFood.name : '(unknown food)',
                    amount: ing.amount,
                    unit: ingredientFood ? (ingredientFood.measurement_unit || '') : '',
                    calories: ingredientFood ? ingredientFood.calories : 0,
                    protein: ingredientFood ? ingredientFood.protein : 0,
                    carbs: ingredientFood ? ingredientFood.carbs : 0,
                    fat: ingredientFood ? ingredientFood.fat : 0,
                    measurement_amount: ingredientFood ? (ingredientFood.measurement_amount || 100) : 100,
                });
            }
        }
        updateIngredientList();
    } else {
        // Populate macros
        form.calories.value = food.calories;
        form.protein.value = food.protein;
        form.carbs.value = food.carbs;
        form.fat.value = food.fat;

        // Populate serving size
        const macrosSection = document.getElementById('macros-section');
        macrosSection.querySelector('[name="measurement_amount"]').value = food.measurement_amount || 1;
        macrosSection.querySelector('[name="measurement_unit"]').value = food.measurement_unit || 'serving';

        // Populate nutrients
        document.getElementById('nutrients-container').textContent = '';
        if (food.nutrients && food.nutrients.length > 0) {
            food.nutrients.forEach(n => addNutrientRow(n.name, n.amount, n.unit));
        }
    }

    // Scroll to form
    form.scrollIntoView({ behavior: 'smooth' });
}

function cancelEdit() {
    editingFoodId = null;
    const form = document.getElementById('create-food-form');
    form.reset();
    document.getElementById('form-heading').textContent = 'Add New Food';
    document.getElementById('form-submit-btn').textContent = 'Add Food';
    document.getElementById('cancel-edit-btn').style.display = 'none';
    document.getElementById('nutrients-container').textContent = '';
    document.getElementById('recipe-servings').value = 1;
    document.getElementById('recipe-totals').style.display = 'none';
    document.querySelectorAll('[name="measurement_amount"]').forEach(el => { el.value = 1; });
    document.querySelectorAll('[name="measurement_unit"]').forEach(el => { el.value = 'serving'; });
    recipeIngredients = [];
    updateIngredientList();
    document.getElementById('type-food').checked = true;
    toggleFoodType();
    resetIngredientPicker();
}

// --- Toggle Logic ---

function toggleFoodType() {
    const type = document.querySelector('input[name="type"]:checked').value;
    const macrosSection = document.getElementById('macros-section');
    const ingredientsSection = document.getElementById('ingredients-section');
    const inputs = macrosSection.querySelectorAll('input');

    if (type === 'recipe') {
        macrosSection.style.display = 'none';
        ingredientsSection.style.display = 'block';
        inputs.forEach(i => i.removeAttribute('required'));
    } else {
        macrosSection.style.display = 'block';
        ingredientsSection.style.display = 'none';
        inputs.forEach(i => i.setAttribute('required', ''));
    }
}

function updateIngredientList() {
    const list = document.getElementById('ingredients-list');
    const buffer = new DOMBuffer(list);
    recipeIngredients.forEach((ing, index) => {
        const div = document.createElement('div');
        div.className = 'nutrient-row nutrient-row--ingredient';

        const nameSpan = document.createElement('span');
        nameSpan.textContent = ing.name;

        const amountSpan = document.createElement('span');
        amountSpan.textContent = `${ing.amount} ${ing.unit}`;

        const factor = ing.amount / (ing.measurement_amount || 100);
        const macroSpan = document.createElement('span');
        macroSpan.className = 'ingredient-macro-label';
        macroSpan.textContent = `${Math.round(ing.calories * factor)} kcal · P:${Math.round(ing.protein * factor)} C:${Math.round(ing.carbs * factor)} F:${Math.round(ing.fat * factor)}`;

        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'btn btn-danger btn-sm';
        removeBtn.textContent = '×';
        removeBtn.onclick = () => removeIngredient(index);

        div.appendChild(nameSpan);
        div.appendChild(amountSpan);
        div.appendChild(macroSpan);
        div.appendChild(removeBtn);
        buffer.append(div);
    });
    buffer.clearAndFlush();
    updateRecipeTotals();
}

function makeMacroLine(label, cal, p, c, f) {
    const div = document.createElement('div');
    const strong = document.createElement('strong');
    strong.textContent = label;
    div.appendChild(strong);
    div.appendChild(document.createTextNode(
        ` ${Math.round(cal)} kcal | P: ${Math.round(p)}g | C: ${Math.round(c)}g | F: ${Math.round(f)}g`
    ));
    return div;
}

function updateRecipeTotals() {
    const totalsDiv = document.getElementById('recipe-totals');
    if (recipeIngredients.length === 0) {
        totalsDiv.style.display = 'none';
        return;
    }

    let cal = 0, p = 0, c = 0, f = 0;
    recipeIngredients.forEach(ing => {
        const factor = ing.amount / (ing.measurement_amount || 100);
        cal += ing.calories * factor;
        p += ing.protein * factor;
        c += ing.carbs * factor;
        f += ing.fat * factor;
    });

    const servings = parseFloat(document.getElementById('recipe-servings').value) || 1;
    totalsDiv.style.display = 'block';
    totalsDiv.textContent = '';
    totalsDiv.appendChild(makeMacroLine('Total:', cal, p, c, f));
    totalsDiv.appendChild(makeMacroLine(`Per serving (${servings}):`, cal / servings, p / servings, c / servings, f / servings));
}

function removeIngredient(index) {
    recipeIngredients.splice(index, 1);
    updateIngredientList();
}

function resetIngredientPicker() {
    ingredientSelectSeq++; // invalidate any in-flight api.getFood requests
    ingredientPortions = [];
    selectedIngredientFood = null;
    ingredientSearch.clear();
    const portionSelect = document.getElementById('ingredient-portion-select');
    while (portionSelect.firstChild) portionSelect.removeChild(portionSelect.firstChild);
    portionSelect.style.display = 'none';
    document.getElementById('ingredient-amount-unit').textContent = '';
    document.getElementById('ingredient-amount').value = '';
}

function addIngredient() {
    const amountInput = document.getElementById('ingredient-amount');
    const amount = parseFloat(amountInput.value);
    if (!selectedIngredientFood || isNaN(amount) || amount <= 0) {
        showToast("Please select a food and enter a valid amount.", 'error');
        return;
    }
    const food = selectedIngredientFood;
    recipeIngredients.push({
        id: food.id,
        name: food.name,
        amount: amount,
        unit: food.measurement_unit || '',
        calories: food.calories,
        protein: food.protein,
        carbs: food.carbs,
        fat: food.fat,
        measurement_amount: food.measurement_amount || 100,
    });
    updateIngredientList();
    resetIngredientPicker();
}

// --- Submit (Create or Update) ---

async function handleSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const type = document.querySelector('input[name="type"]:checked').value;

    const activeSection = document.getElementById(type === 'recipe' ? 'ingredients-section' : 'macros-section');
    const measurement_amount = parseFloat(activeSection.querySelector('[name="measurement_amount"]').value) || 1;
    const measurement_unit = activeSection.querySelector('[name="measurement_unit"]').value.trim() || 'serving';

    let foodData = {
        name: form.name.value,
        type: type,
        measurement_unit,
        measurement_amount,
    };

    if (type === 'recipe') {
        if (recipeIngredients.length === 0) {
            showToast("Please add at least one ingredient for the recipe.", 'error');
            return;
        }
        const servings = parseFloat(document.getElementById('recipe-servings').value) || 1;
        let cal = 0, p = 0, c = 0, fat = 0;
        const ingredientsMap = {};

        recipeIngredients.forEach(ing => {
            ingredientsMap[ing.id] = ing.amount;
            const factor = ing.amount / (ing.measurement_amount || 100);
            cal += ing.calories * factor;
            p += ing.protein * factor;
            c += ing.carbs * factor;
            fat += ing.fat * factor;
        });

        // Store per-serving macros
        foodData.calories = cal / servings;
        foodData.protein = p / servings;
        foodData.carbs = c / servings;
        foodData.fat = fat / servings;
        foodData.servings = servings;
        foodData.ingredients = ingredientsMap;
    } else {
        foodData.calories = parseFloat(form.calories.value);
        foodData.protein = parseFloat(form.protein.value);
        foodData.carbs = parseFloat(form.carbs.value);
        foodData.fat = parseFloat(form.fat.value);

        const nutrients = [];
        document.querySelectorAll('#nutrients-container .nutrient-row').forEach(row => {
            const nameInput = row.querySelector('.nutrient-name');
            if (nameInput) {
                const name = nameInput.value;
                const amount = parseFloat(row.querySelector('.nutrient-amount').value);
                const unit = row.querySelector('.nutrient-unit').value;
                if (name && !isNaN(amount) && unit) {
                    nutrients.push({ name, amount, unit });
                }
            }
        });
        foodData.nutrients = nutrients;
    }

    try {
        if (editingFoodId) {
            await api.updateFood(editingFoodId, foodData);
        } else {
            await api.createFood(foodData);
        }

        // Reset form
        editingFoodId = null;
        form.reset();
        document.getElementById('form-heading').textContent = 'Add New Food';
        document.getElementById('form-submit-btn').textContent = 'Add Food';
        document.getElementById('cancel-edit-btn').style.display = 'none';
        document.getElementById('nutrients-container').textContent = '';
        document.getElementById('recipe-servings').value = 1;
        document.getElementById('recipe-totals').style.display = 'none';
        document.querySelectorAll('[name="measurement_amount"]').forEach(el => { el.value = 1; });
        document.querySelectorAll('[name="measurement_unit"]').forEach(el => { el.value = 'serving'; });
        recipeIngredients = [];
        updateIngredientList();
        document.getElementById('type-food').checked = true;
        toggleFoodType();
        resetIngredientPicker();

        loadFoods();
    } catch (e) {
        showToast("Failed to save food: " + e.message, 'error');
    }
}

window.addEventListener('load', () => {
    loadFoods();
    toggleFoodType();
    document.getElementById('create-food-form').addEventListener('submit', handleSubmit);
    document.getElementById('add-nutrient-btn').addEventListener('click', () => addNutrientRow());
    document.querySelectorAll('input[name="type"]').forEach(radio => {
        radio.addEventListener('change', toggleFoodType);
    });
    document.getElementById('recipe-servings').addEventListener('input', updateRecipeTotals);
    document.getElementById('add-ingredient-btn').addEventListener('click', addIngredient);
    document.getElementById('cancel-edit-btn').addEventListener('click', cancelEdit);
    const ingredientPortionSelect = document.getElementById('ingredient-portion-select');
    const ingredientUnitLabel = document.getElementById('ingredient-amount-unit');

    ingredientPortionSelect.addEventListener('change', () => {
        const portion = ingredientPortions.find(p => p.name === ingredientPortionSelect.value);
        if (portion) {
            document.getElementById('ingredient-amount').value = portion.gram_weight;
            ingredientUnitLabel.textContent = '(g)';
        } else if (selectedIngredientFood) {
            document.getElementById('ingredient-amount').value = '';
            ingredientUnitLabel.textContent = selectedIngredientFood.measurement_unit ? `(${selectedIngredientFood.measurement_unit})` : '';
        }
    });

    ingredientSearch = new FoodSearch(
        document.getElementById('ingredient-search-container'),
        {
            onSelect: async (food) => {
                const seq = ++ingredientSelectSeq;
                ingredientPortions = [];
                while (ingredientPortionSelect.firstChild) ingredientPortionSelect.removeChild(ingredientPortionSelect.firstChild);
                ingredientPortionSelect.style.display = 'none';
                ingredientUnitLabel.textContent = '';

                if (!food) { selectedIngredientFood = null; return; }

                selectedIngredientFood = food;
                ingredientUnitLabel.textContent = food.measurement_unit ? `(${food.measurement_unit})` : '';

                try {
                    const fullFood = await api.getFood(food.id);
                    if (seq !== ingredientSelectSeq) return; // superseded by a later selection
                    selectedIngredientFood = fullFood;
                    ingredientUnitLabel.textContent = fullFood.measurement_unit ? `(${fullFood.measurement_unit})` : '';

                    if (fullFood.portions && fullFood.portions.length > 0) {
                        ingredientPortions = fullFood.portions;
                        ingredientPortionSelect.style.display = '';

                        const defOption = document.createElement('option');
                        defOption.value = '';
                        defOption.textContent = `${fullFood.measurement_amount} ${fullFood.measurement_unit} (base)`;
                        ingredientPortionSelect.appendChild(defOption);

                        fullFood.portions.forEach(p => {
                            const opt = document.createElement('option');
                            opt.value = p.name;
                            opt.textContent = p.name;
                            ingredientPortionSelect.appendChild(opt);
                        });
                    }
                } catch (err) {
                    if (seq !== ingredientSelectSeq) return;
                    selectedIngredientFood = null;
                    ingredientUnitLabel.textContent = '';
                    showToast("Could not load food details.", 'error');
                }
            }
        }
    );
});
