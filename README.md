## Kubernetes y cluster de Kafka (3 brokers)

Prueba de concepto,  desplegar un cluster Kafka, con 3 brokers, sobre Kind Kubernetes. Darle "permanencia" al cluster de Kafka. Además, implementar dos "micro-servicios" para utilizar el cluster, accesibles desde el host mediante Ingress. Permite crear tópicos (replicados en los tres nodos), enviar y recibir mensajes de esos tópicos.

El deployment se realiza con un sólo comando, comandos.bat (en Windows, o aunque debe funcionar, con pequeños cambios, en Linux y MAC). Algunos comentarios:

- Empezamos por desplegar el cluster de Kubernetes usando Kind, con 4 nodos, un control-plane y tres workers. Se agregan opciones de Kind para desplegar posteriormente Ingress y darle almacenamiento permanente, en directorios de nuestra computadora, a los pods del cluster. El mecanismo es similar al que usamos para crear un cluster de MongoDB: (https://github.com/avaco2312/kubernetes-mongodb-replicaset)
- Creamos un deployment para Zookeeper (1 réplica) y para Kafka (3 réplicas) usando las imágenes de bitnami. Definimos las variables de ambiente necesarias para la inicialización como cluster con tres brokers de Kafka.
- Se le asigna almacenamiento permanente al pod de Zookeeper y a cada pod de Kafka, en subdirectorios de nuetra computadora. Para el detalle, remitirse al ejemplo de MongoDB: (https://github.com/avaco2312/kubernetes-mongodb-replicaset)
- Se crea el servicio para acceder, dentro del cluster, a Zookeeper y el correspondiente a Kafka (que balancea entre los tres brokers)
- Se crea el deployment y el servicio para un cliente Go que envía mensajes al cluster Kafka. El código y Dockerfile en goclient/producer. Este producer escucha en el puerto 8070 y tiene dos funciones. Con el path topic/xxx, crea el tópico xxx, y con el path producer/xxx envía al tópico xxx lo que contenga el body de la petición. El tópico se crea con una partición y 3 réplicas, lo que garantiza la resiliencia ante un fallo en los brokers de Kafka.
- Se crea el deployment y el servicio para un cliente Go que solicita un mensaje a Kafka, de un tópico específico. Para ello escucha en el puerto 8072, con el path /consumer/xxx para recibir un mensaje del tópico xxx (si existe). El códico en go-client/consumer.
- Internamente el consumer implementa dos mecanismos, si el tópico es primera vez que se solicita, crea un channel de Go para él y además una goroutine que solicita "eternamente" los mensajes de ese tópico y los envía al channel. Una vez inicializado el tópico, se lee del channel el próximo mensaje (si no existe, se interrumpe mediante un timeout). También, si el tópico no está creado, antes de "escucharlo" lo crea, para garantizar que se cree con 3 réplicas.
- Desplegamos Ingress y asignamos a localhost los path "/consumer", "/topic" y "/producer" para recibir mensajes, crear un tópico o enviar un mensaje, respectivamente. 
- Hora de probar, crear un tópico:
```
    curl -X POST http://localhost/topic/primertopico

    HTTP/1.1 200 OK
    Date: Thu, 04 Nov 2021 01:26:43 GMT
    Content-Length: 0
    Connection: close
```
- Enviar un mensaje:
```
    curl -X POST http://localhost/producer/primertopico
    -d "Primer mensaje enviado"

    HTTP/1.1 200 OK
    Date: Thu, 04 Nov 2021 01:28:54 GMT
    Content-Length: 0
    Connection: close
```    
- Leer un mensaje:
```
    curl -X GET http://localhost/consumer/primertopico

    HTTP/1.1 200 OK
    Date: Thu, 04 Nov 2021 01:35:16 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 53
    Connection: close

    offset: 0 key: address-10.244.0.5:50808 value: Primer mensaje enviado
```
- Si enviamos varios mensajes y los leemos, observe como el offset del mensaje en la cola de Kafka va incrementándose, de uno en uno.
- En la lista de comandos está comentada la línea para desplegar kafka-manager, que es una interfase web que permite ver el estado de nuestro cluster Kafka. Pueden probar a activarlo, hacer un port-forward de su servicio en el puerto 9000 y acceder desde el navegador de nuestra computadora.
