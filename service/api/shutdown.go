/*	DA FARE SE UTILIZZIAMO GOROUTINE (threads)
	Questo file definisce il metodo Close() da chiamare al momento dello spegnimento del server.
	Il suo scopo è quello di chiudere correttamente tutte le risorse aperte evitando la perdita di dati.

	Dobbiamo modificare la funzione per implementare la terminazione esplicita di:
		Tutte le gorutine
		Tutte le connessioni

	Attualmente la funzione restituisce solo nil, indicando che nessuna risorsa è stata chiusa.
	L'implementazione sarà necessaria se nel progetto verranno aggiunte goroutine o servizi
	che richiedono una terminazione esplicita.
*/

/* Attualmente è vuoto.
Va implementato per chiudere correttamente tutte le risorse quando il server viene spento.
*/

package api

// Close should close everything opened in the lifecycle of the `_router`; for example, background goroutines.
func (rt *_router) Close() error {
	return nil
}
