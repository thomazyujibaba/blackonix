package plugins

import (
	"context"
	"fmt"
)

// CheckStockTool é um mock que simula consulta de estoque de produtos.
type CheckStockTool struct{}

func NewCheckStockTool() *CheckStockTool {
	return &CheckStockTool{}
}

func (t *CheckStockTool) Name() string {
	return "check_stock"
}

func (t *CheckStockTool) Description() string {
	return "Consulta o estoque de um produto em uma loja específica. Retorna a quantidade disponível."
}

func (t *CheckStockTool) ParametersSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"product_name": map[string]interface{}{
				"type":        "string",
				"description": "Nome do produto a consultar",
			},
			"store_id": map[string]interface{}{
				"type":        "string",
				"description": "ID ou nome da loja",
			},
		},
		"required": []string{"product_name"},
	}
}

func (t *CheckStockTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	productName, _ := params["product_name"].(string)
	storeID, _ := params["store_id"].(string)

	if productName == "" {
		return "", fmt.Errorf("product_name is required")
	}

	if storeID == "" {
		storeID = "loja-principal"
	}

	// Mock de dados de estoque
	stock := map[string]int{
		"caixa de presente":    45,
		"papel de seda":        120,
		"fita decorativa":      78,
		"sacola kraft":         200,
		"caixa para doces":     33,
		"embalagem para vinho": 15,
	}

	qty, found := stock[productName]
	if !found {
		return fmt.Sprintf("Produto '%s' não encontrado no catálogo da loja '%s'.", productName, storeID), nil
	}

	return fmt.Sprintf("Produto: %s | Loja: %s | Estoque: %d unidades disponíveis.", productName, storeID, qty), nil
}
