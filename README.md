Ingress admission controller which mutates and validates Ingress objects requests to OCP server so they can be picked up by cert-manager so issuing ceritificates can be served. 

- Host name and common name are mutated so they follow this naming convention: <service>-<namespace>.apps.<ClusterBaseDomain>
- Annotations are mutated to reflect the environment variables set in deployment of server which are set up to point to the CAS issuer and cert-manager (These can be changed through changing environment variables within deployment to reflect the specific needs of the cluster)
- User must first add Annotations and choose which termination type they want: edge or passthrough
- Services within the namespace retrieved are checked against the Ingress object to ensure user is attaching the ingress to an existing running service
- Further validation occurs in the form of checking for spec such as tls, hosts and secretName fields and values exist


INSTRUCTION ON HOW TO DEPLOY:

oc apply -f configs/ (run twice)
oc new-build --name admission-controller-ingress --binary=true --strategy=docker -n admission-namespace
oc start-build admission-controller-ingress --from-dir=. --follow -n admission-namespace# cert-admission-ctrl
