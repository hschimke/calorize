import { api } from './api.js';

export class FoodSearch {
    /**
     * @param {HTMLElement} containerEl
     * @param {{ onSelect: function, placeholder?: string }} options
     */
    constructor(containerEl, options = {}) {
        this.containerEl = containerEl;
        this.onSelect = options.onSelect || (() => {});
        this.placeholder = options.placeholder || 'Search foods...';

        this.recentFoods = [];
        this._debounceTimer = null;
        this._activeIndex = -1;
        this._renderedFoods = new Map();
        this._outsideClickListener = null;

        this._buildDOM();
        this._attachEvents();
        this._loadRecent();
    }

    // ------------------------------------------------------------------ DOM

    _buildDOM() {
        this._wrapper = document.createElement('div');
        this._wrapper.className = 'food-search';

        this._input = document.createElement('input');
        this._input.type = 'text';
        this._input.className = 'food-search-input';
        this._input.placeholder = this.placeholder;

        this._clearBtn = document.createElement('span');
        this._clearBtn.className = 'food-search-clear';
        this._clearBtn.textContent = '\u2715'; // ✕

        this._dropdown = document.createElement('ul');
        this._dropdown.className = 'food-search-dropdown';

        this._wrapper.appendChild(this._input);
        this._wrapper.appendChild(this._clearBtn);
        this._wrapper.appendChild(this._dropdown);
        this.containerEl.appendChild(this._wrapper);
    }

    // ------------------------------------------------------------------ Events

    _attachEvents() {
        this._input.addEventListener('focus', () => this._openDropdown());
        this._input.addEventListener('input', () => this._onInput());
        this._input.addEventListener('keydown', (e) => this._onKeydown(e));
        this._clearBtn.addEventListener('click', () => this._onClear());

        this._outsideClickListener = (e) => {
            if (!this._wrapper.contains(e.target)) {
                this._closeDropdown();
            }
        };
        document.addEventListener('click', this._outsideClickListener);
    }

    // ------------------------------------------------------------------ Data loading

    async _loadRecent() {
        try {
            const results = await api.getFoods({ recent: 'true' });
            this.recentFoods = Array.isArray(results) ? results : [];
        } catch (e) {
            this.recentFoods = [];
        }
        this._renderRecent();
    }

    // ------------------------------------------------------------------ Render helpers

    _createSectionHeader(label) {
        const li = document.createElement('li');
        li.className = 'food-search-section';
        li.textContent = label;
        return li;
    }

    _createFoodItem(food) {
        const li = document.createElement('li');
        li.className = 'food-search-item';
        li.dataset.foodId = food.id;

        const nameSpan = document.createElement('span');
        nameSpan.textContent = food.name;

        const calSpan = document.createElement('span');
        calSpan.className = 'food-search-item-calories';
        calSpan.textContent = Math.round(food.calories) + ' kcal';

        li.appendChild(nameSpan);
        li.appendChild(calSpan);

        li.addEventListener('click', () => this._selectFood(food));

        return li;
    }

    _createNoRecentItem() {
        const li = document.createElement('li');
        li.className = 'food-search-item';
        li.style.pointerEvents = 'none';

        const span = document.createElement('span');
        span.textContent = 'No recent foods';
        span.style.color = 'var(--color-text-muted)';

        li.appendChild(span);
        return li;
    }

    /**
     * Clear and re-render the dropdown from provided arrays.
     * Maintains this._renderedFoods map for keyboard-Enter lookup.
     * @param {Array} recentItems — food objects for the "Recent" section
     * @param {Array} [moreItems]  — food objects for the "More results" section
     */
    _renderDropdown(recentItems, moreItems) {
        // Remove all children (no innerHTML)
        while (this._dropdown.firstChild) {
            this._dropdown.removeChild(this._dropdown.firstChild);
        }
        this._activeIndex = -1;
        this._renderedFoods = new Map();

        // Recent section
        this._dropdown.appendChild(this._createSectionHeader('Recent'));
        if (recentItems.length === 0) {
            this._dropdown.appendChild(this._createNoRecentItem());
        } else {
            for (const food of recentItems) {
                this._renderedFoods.set(food.id, food);
                this._dropdown.appendChild(this._createFoodItem(food));
            }
        }

        // More results section
        if (moreItems && moreItems.length > 0) {
            this._dropdown.appendChild(this._createSectionHeader('More results'));
            for (const food of moreItems) {
                this._renderedFoods.set(food.id, food);
                this._dropdown.appendChild(this._createFoodItem(food));
            }
        }
    }

    _renderRecent() {
        this._renderDropdown(this.recentFoods);
    }

    // ------------------------------------------------------------------ Input handling

    _onInput() {
        const query = this._input.value;

        if (query === '') {
            clearTimeout(this._debounceTimer);
            this._renderRecent();
            this._openDropdown();
            return;
        }

        // Client-side filter of recent foods shown immediately
        const filteredRecent = this.recentFoods.filter(
            f => f.name.toLowerCase().includes(query.toLowerCase())
        );
        this._renderDropdown(filteredRecent);
        this._openDropdown();

        // Debounce server call for queries >= 2 chars
        clearTimeout(this._debounceTimer);
        if (query.length >= 2) {
            const capturedQuery = query;
            this._debounceTimer = setTimeout(async () => {
                // Guard: skip if input changed or too short
                if (this._input.value !== capturedQuery || capturedQuery.length < 2) {
                    return;
                }
                try {
                    const results = await api.getFoods({ q: capturedQuery });
                    // Guard again after async resolves
                    if (this._input.value !== capturedQuery) return;

                    const serverResults = Array.isArray(results) ? results : [];
                    const currentQuery = this._input.value;

                    const visibleRecentIds = new Set(
                        this.recentFoods
                            .filter(f => f.name.toLowerCase().includes(currentQuery.toLowerCase()))
                            .map(f => f.id)
                    );

                    const moreItems = serverResults.filter(f => !visibleRecentIds.has(f.id));

                    this._renderDropdown(
                        this.recentFoods.filter(f => f.name.toLowerCase().includes(currentQuery.toLowerCase())),
                        moreItems.length > 0 ? moreItems : undefined
                    );
                    this._openDropdown();
                } catch (e) {
                    // Ignore network errors during search
                }
            }, 200);
        }
    }

    // ------------------------------------------------------------------ Selection

    _selectFood(food) {
        this._input.value = food.name;
        this._closeDropdown();
        this._clearBtn.classList.add('visible');
        this.onSelect(food);
    }

    _onClear() {
        this._input.value = '';
        this._clearBtn.classList.remove('visible');
        clearTimeout(this._debounceTimer);
        this._renderRecent();
        this.onSelect(null);
        this._openDropdown();
    }

    // ------------------------------------------------------------------ Keyboard navigation

    _onKeydown(e) {
        // Selectable items have data-food-id (excludes "No recent foods" placeholder)
        const selectableItems = Array.from(
            this._dropdown.querySelectorAll('.food-search-item[data-food-id]')
        );

        switch (e.key) {
            case 'ArrowDown': {
                e.preventDefault();
                if (!this._dropdown.classList.contains('open')) {
                    this._openDropdown();
                    return;
                }
                if (selectableItems.length === 0) return;
                this._activeIndex = (this._activeIndex + 1) % selectableItems.length;
                this._updateActiveClass(selectableItems);
                break;
            }
            case 'ArrowUp': {
                e.preventDefault();
                if (selectableItems.length === 0) return;
                if (this._activeIndex <= 0) {
                    this._activeIndex = selectableItems.length - 1;
                } else {
                    this._activeIndex = this._activeIndex - 1;
                }
                this._updateActiveClass(selectableItems);
                break;
            }
            case 'Enter': {
                e.preventDefault();
                if (this._activeIndex >= 0 && selectableItems[this._activeIndex]) {
                    const foodId = selectableItems[this._activeIndex].dataset.foodId;
                    const food = this._renderedFoods.get(foodId);
                    if (food) this._selectFood(food);
                }
                break;
            }
            case 'Escape': {
                this._closeDropdown();
                this._activeIndex = -1;
                this._clearActiveClass(selectableItems);
                break;
            }
        }
    }

    _updateActiveClass(selectableItems) {
        for (let i = 0; i < selectableItems.length; i++) {
            if (i === this._activeIndex) {
                selectableItems[i].classList.add('active');
            } else {
                selectableItems[i].classList.remove('active');
            }
        }
    }

    _clearActiveClass(selectableItems) {
        const items = selectableItems || Array.from(
            this._dropdown.querySelectorAll('.food-search-item.active')
        );
        for (const item of items) {
            item.classList.remove('active');
        }
    }

    // ------------------------------------------------------------------ Dropdown open/close

    _openDropdown() {
        this._dropdown.classList.add('open');
    }

    _closeDropdown() {
        this._dropdown.classList.remove('open');
    }

    // ------------------------------------------------------------------ Public API

    /**
     * Clear input and selection, re-show recent list, open dropdown.
     */
    reset() {
        this._input.value = '';
        this._clearBtn.classList.remove('visible');
        clearTimeout(this._debounceTimer);
        this._activeIndex = -1;
        this._renderRecent();
        this._openDropdown();
    }

    /**
     * Remove all DOM created by this instance and detach listeners.
     */
    destroy() {
        if (this._outsideClickListener) {
            document.removeEventListener('click', this._outsideClickListener);
            this._outsideClickListener = null;
        }
        clearTimeout(this._debounceTimer);
        if (this._wrapper && this._wrapper.parentNode) {
            this._wrapper.parentNode.removeChild(this._wrapper);
        }
    }
}
